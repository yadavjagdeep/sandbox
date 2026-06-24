package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"

	"view-counter/models"
	"view-counter/store"
)

const BatchSize = 100

func main() {
	ctx := context.Background()

	db, err := sql.Open("postgres", "host=localhost port=5432 user=views password=views dbname=views sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	pgStore := store.NewPostgresStore(db)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "valid-views",
		GroupID:  "counter",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	fmt.Println("Counter worker started. Consuming valid-views...")

	// Aggregate counts per video
	counts := make(map[string]int64)

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		var event models.ViewEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			continue
		}

		counts[event.VideoID]++

		if totalCount(counts) >= BatchSize {
			for videoID, count := range counts {
				if err := pgStore.IncrementCount(videoID, count); err != nil {
					log.Printf("increment error for %s: %v", videoID, err)
				} else {
					fmt.Printf("Incremented %s by +%d\n", videoID, count)
				}
			}

			counts = make(map[string]int64)
		}
	}
}

func totalCount(m map[string]int64) int64 {
	var total int64
	for _, v := range m {
		total += v
	}
	return total
}
