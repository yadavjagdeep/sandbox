package tier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"multi-tier-storage/models"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const BucketName = "orders-archive"

type ColdStore struct {
	client *s3.Client
}

func NewColdStore(client *s3.Client) *ColdStore {
	return &ColdStore{client: client}
}

func (s *ColdStore) PutOrder(ctx context.Context, doc models.OrderDocument) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("orders/%s.json", strconv.FormatInt(doc.ID, 10))
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *ColdStore) GetOrder(ctx context.Context, id int64) (*models.OrderDocument, error) {
	key := fmt.Sprintf("orders/%s.json", strconv.FormatInt(id, 10))

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	var doc models.OrderDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}
