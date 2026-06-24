package main

import (
	"context"
	"database/sql"
	"flash-sale/gate"
	"flash-sale/store"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

const HoldDuration = 10 * time.Minute

func main() {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=flashsale password=flashsale dbname=flashsale sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	pgStore := store.NewPostgresStore(db)
	redisGate := gate.NewRedisGate("localhost:6379")

	fmt.Println("Releaser started. Checking expired holds every 30 seconds...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		released, err := pgStore.ReleaseExpireHolds(HoldDuration)
		if err != nil {
			log.Printf("release error: %v", err)
			continue
		}

		if len(released) > 0 {
			ctx := context.Background()
			for _, itemID := range released {
				redisGate.Release(ctx, itemID)
			}

			fmt.Printf("Released %d expired holds\n", len(released))
		}
	}
}
