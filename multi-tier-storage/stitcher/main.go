package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	_ "github.com/lib/pq"

	"multi-tier-storage/tier"
)

func main() {
	// PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5432 user=orders password=orders dbname=orders sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	// DynamoDB
	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://localhost:8000")
	})

	hotStore := tier.NewHotStore(db)
	warmStore := tier.NewWarmStore(dynamoClient)

	// Find orders older than 6 months
	cutoff := time.Now().Add(-tier.HotThreshold)
	rows, err := db.Query("SELECT id FROM orders WHERE created_at < $1", cutoff)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var demoted int
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			log.Printf("scan error: %v", err)
			continue
		}

		// Get full order from SQL
		doc, err := hotStore.GetOrder(id)
		if err != nil {
			log.Printf("get order %d: %v", id, err)
			continue
		}

		// Set TTL (2 years from now)
		doc.TTL = time.Now().Add(tier.WarmThreshold).Unix()

		// Write to DynamoDB
		if err := warmStore.PutOrder(ctx, *doc); err != nil {
			log.Printf("put warm %d: %v", id, err)
			continue
		}

		// NOTE: We do NOT delete from SQL here.
		// TTL-based deletion handles that separately.
		// There may be a brief gap where data exists in both tiers — that's acceptable.

		demoted++
	}

	fmt.Printf("Stitcher complete: demoted %d orders from hot → warm\n", demoted)
}
