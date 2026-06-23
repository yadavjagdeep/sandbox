package main

import (
	"context"
	"database/sql"
	"fmt"
	"job-schedular/store"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

type ScheduleRequest struct {
	Command    any    `json:"command"`
	ScheduleAt string `json:"scheduled_at"`
	CronExpr   string `json:"cron_expr,omitempty"`
}

func main() {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=scheduler password=scheduler dbname=scheduler sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	s := store.NewPostgresStore(db)

	e := echo.New()

	e.POST("/jobs", func(c echo.Context) error {
		var req ScheduleRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}

		scheduledAt, err := time.Parse(time.RFC3339, req.ScheduleAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid scheduled_at format, use RFC3339"})
		}

		id, err := s.InsertJob(req.Command, scheduledAt)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusCreated, map[string]any{
			"job_id":       id,
			"scheduled_at": scheduledAt,
			"status":       "scheduled",
		})
	})
	e.POST("/jobs/recurring", func(c echo.Context) error {
		var req ScheduleRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}

		if req.CronExpr == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "cron_expr required"})
		}

		id, err := s.InsertRecurringJob(req.Command, req.CronExpr)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusCreated, map[string]any{
			"recurring_job_id": id,
			"cron_expr":        req.CronExpr,
			"status":           "active",
		})
	})
	e.GET("/health", func(c echo.Context) error {
		ctx := context.Background()
		awsCfg, _ := config.LoadDefaultConfig(ctx,
			config.WithRegion("us-east-1"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
		sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String("http://localhost:9324")
		})
		result, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
			QueueName: aws.String("jobs"),
		})
		if err != nil {
			return c.JSON(http.StatusOK, map[string]string{"db": "ok", "sqs": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"db": "ok", "sqs_queue": *result.QueueUrl})
	})

	fmt.Println("Job scheduler API on :8080")
	e.Logger.Fatal(e.Start(":8080"))

}
