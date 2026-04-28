package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
)

type HashtagState struct {
	Count  int64
	Top100 json.RawMessage
}

var (
	state   = make(map[string]*HashtagState)
	stateMu sync.RWMutex
)

func main() {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=5440 user=hashtag password=hashtag123 dbname=hashtag sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.Exec(`CREATE TABLE IF NOT EXISTS hashtags (
		name TEXT PRIMARY KEY,
		photo_count BIGINT DEFAULT 0,
		top_photos JSONB NOT NULL DEFAULT '[]',
		updated_at TIMESTAMP DEFAULT NOW()
	)`)

	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer([]string{"127.0.0.1:9092"}, config)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()

	partitions, err := consumer.Partitions("posts-by-hashtag")
	if err != nil {
		panic(err)
	}

	// Periodic flush
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			flushToPostgres(db)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Consumer running: processing posts-by-hashtag")

	// Consume all partitions
	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition("posts-by-hashtag", partition, sarama.OffsetNewest)
		if err != nil {
			panic(err)
		}
		defer pc.Close()

		go func(pc sarama.PartitionConsumer) {
			for msg := range pc.Messages() {
				var data map[string]any
				json.Unmarshal(msg.Value, &data)

				hashtag := data["hashtag"].(string)
				top100, _ := json.Marshal(data["top_100_posts"])

				stateMu.Lock()
				if state[hashtag] == nil {
					state[hashtag] = &HashtagState{}
				}
				state[hashtag].Count++
				state[hashtag].Top100 = top100
				stateMu.Unlock()

				fmt.Printf("processed: %s (count: %d)\n", hashtag, state[hashtag].Count)
			}
		}(pc)
	}

	<-signals
	fmt.Println("flushing before shutdown...")
	flushToPostgres(db)
}

func flushToPostgres(db *sql.DB) {
	stateMu.Lock()
	defer stateMu.Unlock()

	for name, hs := range state {
		_, err := db.Exec(`
			INSERT INTO hashtags (name, photo_count, top_photos, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (name) DO UPDATE SET
				photo_count = hashtags.photo_count + $2,
				top_photos = $3,
				updated_at = NOW()
		`, name, hs.Count, hs.Top100)
		if err != nil {
			fmt.Printf("flush error for %s: %v\n", name, err)
		}
	}
	fmt.Printf("flushed %d hashtags to postgres\n", len(state))

	for _, hs := range state {
		hs.Count = 0
	}
}
