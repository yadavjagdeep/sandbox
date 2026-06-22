package main

import (
	"context"
	"fmt"
	"log"
	"recent-searches/handler"
	"recent-searches/store"

	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
)

func main() {
	redisStore := store.NewRedisStore("localhost:6379")
	if err := redisStore.Ping(context.Background()); err != nil {
		log.Fatalf("redis: %v", err)
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "user-searches",
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	h := handler.NewSearchHandler(redisStore, writer)

	e := echo.New()
	e.POST("/searches", h.PostSearch)
	e.GET("/searches/:userId", h.GetSearches)

	fmt.Println("Recent searches server on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
