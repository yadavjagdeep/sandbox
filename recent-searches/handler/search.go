package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"recent-searches/store"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
)

type SearchHandler struct {
	redis  *store.RedisStore
	writer *kafka.Writer
}

func NewSearchHandler(redis *store.RedisStore, writer *kafka.Writer) *SearchHandler {
	return &SearchHandler{
		redis:  redis,
		writer: writer,
	}
}

type SearchRequest struct {
	UserID string `json:"user_id"`
	Query  string `json:"query"`
}

type SearchEvent struct {
	UserID     string `json:"user_id"`
	Query      string `json:"query"`
	SearchedAt int64  `json:"searched_at"`
}

func (h *SearchHandler) PostSearch(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.UserID == "" || req.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "user_id and query is required"})
	}

	ctx := context.Background()

	// 1. write to redis
	if err := h.redis.AddSearch(ctx, req.UserID, req.Query); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "redis write failed"})
	}

	// 2. Produce to kafka (async for dynamoDB persiatance)

	event := SearchEvent{
		UserID:     req.UserID,
		Query:      req.Query,
		SearchedAt: time.Now().UnixMilli(),
	}
	eventBytes, _ := json.Marshal(event)

	go func() {
		h.writer.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(req.UserID),
			Value: eventBytes,
		})
	}()

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SearchHandler) GetSearches(c echo.Context) error {
	userID := c.Param("userId")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	ctx := context.Background()
	searches, err := h.redis.GetRecent(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "redis read failed"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":  userID,
		"searches": searches,
	})
}
