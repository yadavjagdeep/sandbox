package queue

import (
	"context"
	"encoding/json"
	"job-schedular/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSQueue struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSQueue(client *sqs.Client, queueURL string) *SQSQueue {
	return &SQSQueue{
		client:   client,
		queueURL: queueURL,
	}
}

func (q *SQSQueue) Push(ctx context.Context, job models.Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(body)),
	})

	return err
}

func (q *SQSQueue) PushBatch(ctx context.Context, jobs []models.Job) error {
	var entries []types.SendMessageBatchRequestEntry
	for i, job := range jobs {
		body, _ := json.Marshal(job)
		entries = append(entries, types.SendMessageBatchRequestEntry{
			Id:          aws.String(string(rune('0' + i))),
			MessageBody: aws.String(string(body)),
		})

		// SQS batch limit is 10
		if len(entries) == 10 || i == len(jobs)-1 {
			_, err := q.client.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{
				QueueUrl: aws.String(q.queueURL),
				Entries:  entries,
			})
			if err != nil {
				return err
			}
			entries = entries[:0]
		}
	}
	return nil
}

func (q *SQSQueue) Pull(ctx context.Context, maxMessage int) ([]models.Job, []string, error) {
	result, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: int32(maxMessage),
		WaitTimeSeconds:     5,
	})
	if err != nil {
		return nil, nil, err
	}
	var jobs []models.Job
	var receiptHandles []string

	for _, msg := range result.Messages {
		var job models.Job
		if err := json.Unmarshal([]byte(*msg.Body), &job); err != nil {
			continue
		}
		jobs = append(jobs, job)
		receiptHandles = append(receiptHandles, *msg.ReceiptHandle)
	}

	return jobs, receiptHandles, nil
}

func (q *SQSQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
