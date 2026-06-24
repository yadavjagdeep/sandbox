package store

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"view-counter/models"

	"github.com/redis/go-redis/v9"
)

const CacheTTL = 1 * time.Minute

type CacheStore struct {
	client   *redis.Client
	postgres *PostgresStore
}

func NewCacheStore(addr string, postgres *PostgresStore) *CacheStore {
	return &CacheStore{
		client:   redis.NewClient(&redis.Options{Addr: addr}),
		postgres: postgres,
	}
}

func (s *CacheStore) key(videoID string) string {
	return "views:" + videoID
}

func (s *CacheStore) GetCount(ctx context.Context, videoID string) (*models.VideoCount, error) {
	val, err := s.client.Get(ctx, s.key(videoID)).Result()
	if err == nil {
		count, _ := strconv.ParseInt(val, 10, 64)
		return &models.VideoCount{VideoID: videoID, ViewCount: count}, nil
	}

	// cache miss - read deom DB
	vc, err := s.postgres.GetCount(videoID)
	if err != nil {
		return nil, err
	}

	s.client.Set(ctx, s.key(videoID), fmt.Sprintf("%d", vc.ViewCount), CacheTTL)
	return vc, nil
}

func (s *CacheStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
