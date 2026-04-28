package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{"127.0.0.1:9092"}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	hashtags := []string{"#sunset", "#travel", "#food", "#coding", "#music"}

	fmt.Println("Producing messages...")
	for i := 0; i < 100; i++ {
		userID := rand.Intn(1000)
		hashtag := hashtags[rand.Intn(len(hashtags))]

		msg := map[string]any{
			"user_id":       userID,
			"hashtag":       hashtag,
			"photo_id":      fmt.Sprintf("photo_%d", rand.Intn(100000)),
			"top_100_posts": generateTop100(),
		}
		data, _ := json.Marshal(msg)

		_, _, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: "posts-by-user",
			Key:   sarama.StringEncoder(strconv.Itoa(userID)),
			Value: sarama.ByteEncoder(data),
		})
		if err != nil {
			fmt.Printf("send error: %v\n", err)
			continue
		}
		fmt.Printf("sent: user=%d hashtag=%s\n", userID, hashtag)
		time.Sleep(100 * time.Millisecond)
	}
}

func generateTop100() []map[string]any {
	posts := make([]map[string]any, 100)
	for i := 0; i < 100; i++ {
		posts[i] = map[string]any{
			"url":   fmt.Sprintf("https://photos.example.com/photo_%d.jpg", rand.Intn(100000)),
			"likes": rand.Intn(10000),
		}
	}
	return posts
}
