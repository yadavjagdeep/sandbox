package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/IBM/sarama"
)

func main() {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Producer.Return.Successes = true

	consumer, err := sarama.NewConsumer([]string{"127.0.0.1:9092"}, config)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()

	producer, err := sarama.NewSyncProducer([]string{"127.0.0.1:9092"}, config)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	partitions, err := consumer.Partitions("posts-by-user")
	if err != nil {
		panic(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Adapter running: posts-by-user → posts-by-hashtag")

	// Consume all partitions
	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition("posts-by-user", partition, sarama.OffsetNewest)
		if err != nil {
			panic(err)
		}
		defer pc.Close()

		go func(pc sarama.PartitionConsumer) {
			for msg := range pc.Messages() {
				var data map[string]any
				json.Unmarshal(msg.Value, &data)

				hashtag := data["hashtag"].(string)

				_, _, err := producer.SendMessage(&sarama.ProducerMessage{
					Topic: "posts-by-hashtag",
					Key:   sarama.StringEncoder(hashtag),
					Value: sarama.ByteEncoder(msg.Value),
				})
				if err != nil {
					fmt.Printf("forward error: %v\n", err)
					continue
				}
				fmt.Printf("forwarded: hashtag=%s\n", hashtag)
			}
		}(pc)
	}

	<-signals
	fmt.Println("shutting down adapter")
}
