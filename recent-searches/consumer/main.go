package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/segmentio/kafka-go"
)

const (
	BatchSize = 25
	TableName = "user_searches"
	TTLMonths = 6
)

type SearchEvent struct {
	UserID     string `json:"user_id"`
	Query      string `json:"query"`
	SearchedAt int64  `json:"searched_at"`
}

func main() {
	ctx := context.Background()

	// DynamoDB client (local)
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://localhost:8000")
	})

	// Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "user-searches",
		GroupID:  "dynamo-writer",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	fmt.Println("Consumer started. Waiting for messages...")

	var batch []SearchEvent

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		var event SearchEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("unmarshal error: %v", err)
			continue
		}

		batch = append(batch, event)

		// Flush when batch is full
		if len(batch) >= BatchSize {
			if err := flushBatch(ctx, dynamoClient, batch); err != nil {
				log.Printf("flush error: %v", err)
			} else {
				fmt.Printf("Flushed %d records to DynamoDB\n", len(batch))
			}
			batch = batch[:0]
		}
	}
}

func flushBatch(ctx context.Context, client *dynamodb.Client, batch []SearchEvent) error {
	var requests []types.WriteRequest

	for _, event := range batch {
		ttl := time.Now().Add(TTLMonths * 30 * 24 * time.Hour).Unix()

		item := map[string]types.AttributeValue{
			"user_id":     &types.AttributeValueMemberS{Value: event.UserID},
			"searched_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(event.SearchedAt, 10)},
			"query":       &types.AttributeValueMemberS{Value: event.Query},
			"ttl":         &types.AttributeValueMemberN{Value: strconv.FormatInt(ttl, 10)},
		}

		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: item},
		})
	}

	_, err := client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			TableName: requests,
		},
	})

	return err
}
