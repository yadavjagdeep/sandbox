package handler

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"multi-tier-storage/models"
	"multi-tier-storage/snowflake"
	"multi-tier-storage/tier"
)

type OrderHandler struct {
	hot       *tier.HotStore
	warm      *tier.WarmStore
	cold      *tier.ColdStore
	generator *snowflake.Generator
}

func NewOrderHandler(hot *tier.HotStore, warm *tier.WarmStore, cold *tier.ColdStore, gen *snowflake.Generator) *OrderHandler {
	return &OrderHandler{
		hot:       hot,
		warm:      warm,
		cold:      cold,
		generator: gen,
	}
}

type CreateOrderRequest struct {
	UserID  int64              `json:"user_id"`
	Items   []models.OrderItem `json:"items"`
	Payment models.Payment     `json:"payment"`
}

func (h *OrderHandler) CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Calculate total
	var total float64
	for _, item := range req.Items {
		total += item.Price * float64(item.Quantity)
	}

	now := time.Now()
	order := models.Order{
		ID:          h.generator.Generate(),
		UserID:      req.UserID,
		Status:      "created",
		TotalAmount: total,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	req.Payment.Amount = total
	req.Payment.Status = "completed"

	if err := h.hot.CreateOrder(order, req.Items, req.Payment); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"order_id": strconv.FormatInt(order.ID, 10),
		"status":   order.Status,
		"total":    order.TotalAmount,
		"tier":     "hot (SQL)",
	})
}

func (h *OrderHandler) GetOrder(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid order id"})
	}

	t := tier.Route(id)
	ctx := context.Background()

	switch t {
	case tier.TierHot:
		doc, err := h.hot.GetOrder(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"tier": "hot (SQL)", "order": doc})

	case tier.TierWarm:
		doc, err := h.warm.GetOrder(ctx, id)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "order not found in warm tier"})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"tier": "warm (DynamoDB)", "order": doc})

	case tier.TierCold:
		return c.JSON(http.StatusGone, map[string]string{
			"error": "order archived, not available for direct access",
			"tier":  "cold (S3)",
		})

	default:
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unknown tier"})
	}
}
