package tier

import (
	"context"
	"fmt"
	"multi-tier-storage/models"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const TableName = "orders_warm"

type WarmStore struct {
	client *dynamodb.Client
}

func NewWarmStore(client *dynamodb.Client) *WarmStore {
	return &WarmStore{client: client}
}

func (s *WarmStore) PutOrder(ctx context.Context, doc models.OrderDocument) error {
	item, err := attributevalue.MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName),
		Item:      item,
	})
	return err
}

func (s *WarmStore) GetOrder(ctx context.Context, id int64) (*models.OrderDocument, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{Value: strconv.FormatInt(id, 10)},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("order not found")
	}

	var doc models.OrderDocument
	if err := attributevalue.UnmarshalMap(result.Item, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}
