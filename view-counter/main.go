package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"

	"view-counter/models"
	"view-counter/store"
)

func main() {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=views password=views dbname=views sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	pgStore := store.NewPostgresStore(db)
	cacheStore := store.NewCacheStore("localhost:6379", pgStore)
	if err := cacheStore.Ping(context.Background()); err != nil {
		log.Fatalf("redis: %v", err)
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "raw-views",
		Balancer: &kafka.Hash{},
	}
	defer writer.Close()

	e := echo.New()

	e.POST("/views", func(c echo.Context) error {
		var event models.ViewEvent
		if err := c.Bind(&event); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}

		if event.VideoID == "" || event.UserID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "video_id and user_id required"})
		}

		event.Timestamp = time.Now().UnixMilli()

		// Partition key: video_id + hash(user_id) % 10
		shard := md5.Sum([]byte(event.UserID))
		shardNum := int(shard[0]) % 10
		partitionKey := fmt.Sprintf("%s_%d", event.VideoID, shardNum)

		body, _ := json.Marshal(event)

		err := writer.WriteMessages(context.Background(), kafka.Message{
			Key:   []byte(partitionKey),
			Value: body,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to queue view"})
		}

		return c.JSON(http.StatusAccepted, map[string]string{"status": "accepted"})
	})

	e.GET("/videos/:id/count", func(c echo.Context) error {
		videoID := c.Param("id")
		ctx := context.Background()

		vc, err := cacheStore.GetCount(ctx, videoID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, vc)
	})

	fmt.Println("View counter API on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
