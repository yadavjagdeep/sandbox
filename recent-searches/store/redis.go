package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	MaxRecent = 10
	KeyTTL    = 30 * 24 * time.Hour
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
	}
}

func (s *RedisStore) Key(userId string) string {
	return "searches:" + userId
}

func (s *RedisStore) AddSearch(ctx context.Context, userId string, query string) error {
	key := s.Key(userId)
	now := float64(time.Now().UnixMilli())

	// ZADD with score = timestamp, If member exists score gets updated and move to top
	err := s.client.ZAdd(ctx, key, redis.Z{
		Score:  now,
		Member: query,
	}).Err()

	if err != nil {
		return err
	}

	// Keep only top 10 (remove everything except the 10 highest score)
	s.client.ZRemRangeByRank(ctx, key, 0, -MaxRecent-1)

	s.client.Expire(ctx, key, KeyTTL) // reset TTL on every write (user is active)

	return nil
}

func (s *RedisStore) GetRecent(ctx context.Context, userId string) ([]string, error) {
	key := s.Key(userId)

	res, err := s.client.ZRevRange(ctx, key, 0, MaxRecent-1).Result()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
