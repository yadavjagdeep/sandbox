package main

import (
	"context"
	"database/sql"
	"flash-sale/gate"
	"flash-sale/store"
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

type BuyRequest struct {
	UserID string `json:"user_id"`
}

type PayRequest struct {
	UserID      string `json:"user_id"`
	InventoryID string `json:"inventory_id"`
}

func main() {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=flashsale password=flashsale dbname=flashsale sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	ctx := context.Background()
	redisGate := gate.NewRedisGate("localhost:6379")
	if err := redisGate.Ping(ctx); err != nil {
		log.Fatalf("redis: %v", err)
	}

	pgStore := store.NewPostgresStore(db)

	// Initialize gate with available inventory count
	count, _ := pgStore.GetAvailableCount("iphone")
	redisGate.InitGate(ctx, "iphone", int64(count))
	fmt.Printf("Gate initialized: iphone = %d slots\n", count)

	e := echo.New()

	e.GET("/sale/items", func(c echo.Context) error {
		remaining, _ := redisGate.GetRemaining(context.Background(), "iphone")
		return c.JSON(http.StatusOK, map[string]any{
			"item": []map[string]any{
				{"item_id": "iphone", "stock": remaining},
			},
		})
	})

	e.POST("sale/items/:id/buy", func(c echo.Context) error {
		itemID := c.Param("id")
		var req BuyRequest
		if err := c.Bind(&req); err != nil || req.UserID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "user_id required"})
		}

		ctx := context.Background()

		// gate check (Redis DECR)
		allowed, err := redisGate.TryEnter(ctx, itemID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "gate error"})
		}

		if !allowed {
			return c.JSON(http.StatusGone, map[string]string{"error": "sold out"})
		}

		item, err := pgStore.ClaimItem(itemID, req.UserID)
		if err != nil {
			redisGate.Release(ctx, itemID)
			return c.JSON(http.StatusGone, map[string]string{"error": "sold out (no stock)"})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"status":       "held",
			"inventory_id": item.ID,
			"message":      "complete payment within 10 minutes",
		})
	})

	e.POST("/sale/items/:id/pay", func(c echo.Context) error {
		var req PayRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}

		if err := pgStore.ConfirmPayment(req.InventoryID, req.UserID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "paid", "message": "purchase confirmed!"})
	})

	fmt.Println("Flash sale API on :8080")
	e.Logger.Fatal(e.Start(":8080"))

}
