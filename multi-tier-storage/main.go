package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/labstack/echo/v4"
    _ "github.com/lib/pq"

    "multi-tier-storage/handler"
    "multi-tier-storage/snowflake"
    "multi-tier-storage/tier"
)

func main() {
    // PostgreSQL
    db, err := sql.Open("postgres", "host=localhost port=5432 user=orders password=orders dbname=orders sslmode=disable")
    if err != nil {
        log.Fatalf("postgres: %v", err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        log.Fatalf("postgres ping: %v", err)
    }

    // AWS config for local services
    ctx := context.Background()
    awsCfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
    )
    if err != nil {
        log.Fatalf("aws config: %v", err)
    }

    // DynamoDB (local)
    dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
        o.BaseEndpoint = aws.String("http://localhost:8000")
    })

    // S3 / MinIO (local)
    s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String("http://localhost:9000")
        o.UsePathStyle = true
    })

    // Stores
    hotStore := tier.NewHotStore(db)
    warmStore := tier.NewWarmStore(dynamoClient)
    coldStore := tier.NewColdStore(s3Client)

    // Snowflake ID generator
    gen := snowflake.New(1)

    // Handler
    h := handler.NewOrderHandler(hotStore, warmStore, coldStore, gen)

    // Server
    e := echo.New()
    e.POST("/orders", h.CreateOrder)
    e.GET("/orders/:id", h.GetOrder)

    fmt.Println("Multi-tier storage server on :8080")
    e.Logger.Fatal(e.Start(":8080"))
}
