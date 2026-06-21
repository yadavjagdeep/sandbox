package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"

	"live-commentry/handler"
	"live-commentry/store"
)

func main() {
	// PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5432 user=cricket password=cricket dbname=cricket sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	// Redis
	redisStore := store.NewRedisStore("localhost:6379")
	if err := redisStore.Ping(context.Background()); err != nil {
		log.Fatalf("redis: %v", err)
	}

	// Stores
	postgresStore := store.NewPostgresStore(db)

	// Handler
	h := handler.NewCommentaryHandler(redisStore, postgresStore)

	// Server
	e := echo.New()

	// Commentator panel (write)
	e.POST("/commentary", h.PostBall)
	e.PUT("/commentary", h.EditBall)

	// User facing (read)
	e.GET("/commentary/:matchId/live", h.GetLive)
	e.GET("/commentary/:matchId/history", h.GetHistory)

	fmt.Println("Live commentary server on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
