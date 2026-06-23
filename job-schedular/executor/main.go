package main

import (
	"context"
	"encoding/json"
	"fmt"
	"job-schedular/models"
	"job-schedular/queue"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	ctx := context.Background()

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

	jobsQueue, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String("jobs")})
	if err != nil {
		log.Fatalf("get jobs queue: %v", err)
	}

	completionQueue, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String("completions")})
	if err != nil {
		log.Fatalf("get completions queue: %v", err)
	}

	q := queue.NewSQSQueue(sqsClient, *jobsQueue.QueueUrl)
	cq := queue.NewSQSQueue(sqsClient, *completionQueue.QueueUrl)

	fmt.Println("Executor started. Waiting for jobs...")

	for {
		jobs, handles, err := q.Pull(ctx, 10)
		if err != nil {
			log.Printf("pull error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for i, job := range jobs {
			startedAt := time.Now()

			fmt.Printf("Executing job %d: %v\n", job.ID, job.Command)
			execute(job)

			completedAt := time.Now()

			// Send completion event
			completion := models.JobCompletion{
				JobID:       job.ID,
				StartedAt:   startedAt,
				CompletedAt: completedAt,
			}
			completionBytes, _ := json.Marshal(completion)
			cq.Push(ctx, models.Job{ID: job.ID, Command: json.RawMessage(completionBytes)})

			// Delete from jobs queue
			q.Delete(ctx, handles[i])

			fmt.Printf("Job %d completed in %v\n", job.ID, completedAt.Sub(startedAt))
		}
	}
}

func execute(job models.Job) {
	cmdBytes, _ := json.Marshal(job.Command)
	fmt.Printf("  Command: %s\n", string(cmdBytes))
	time.Sleep(500 * time.Millisecond) // simulate work
}
