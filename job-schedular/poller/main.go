package main

import (
	"context"
	"database/sql"
	"fmt"
	"job-schedular/queue"
	"job-schedular/store"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

	// PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5432 user=scheduler password=scheduler dbname=scheduler sslmode=disable")
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	s := store.NewPostgresStore(db)

	// SQS
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String("http://localhost:9324")
	})

	queueResult, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String("jobs"),
	})

	if err != nil {
		log.Fatalf("get queue url: %v", err)
	}

	q := queue.NewSQSQueue(sqsClient, *queueResult.QueueUrl)

	fmt.Println("Poller started. Polling every 5 seconds...")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		jobs, err := s.PickDueJobs(100)
		if err != nil {
			log.Printf("pick error: %v", err)
			continue
		}

		if len(jobs) == 0 {
			continue
		}

		if err := q.PushBatch(ctx, jobs); err != nil {
			log.Printf("sqs push error: %v", err)
			continue
		}

		fmt.Printf("Picked and queued %d jobs\n", len(jobs))

		// Replenish recurring jobs
		replenishRecurring(s)

	}

}

func replenishRecurring(s *store.PostgresStore) {
	recurring, err := s.GetActiveRecurringJobs()
	if err != nil {
		log.Printf("get recurring: %v", err)
		return
	}

	for _, rj := range recurring {
		count, err := s.GetPendingRecurringCount(rj.ID)
		if err != nil {
			continue
		}

		if count < 5 {
			// Insert next schedules (simplified: every 5 minutes for demo)
			// In production, parse cron_expr and compute actual next times
			for i := count; i < 5; i++ {
				nextTime := time.Now().Add(time.Duration(i+1) * 5 * time.Minute)
				s.InsertScheduledJob(rj.Command, nextTime, rj.ID)
			}
			fmt.Printf("Replenished recurring job %d: added %d schedules\n", rj.ID, 5-count)
		}
	}
}
