package store

import (
	"context"
	"encoding/json"
	models "live-commentry/model"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	MaxBalls = 15
	MatchTTL = 6 * time.Hour
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

func (s *RedisStore) key(matchID string) string {
	return "commentry:" + matchID
}

func (s *RedisStore) PushBall(ctx context.Context, ball models.Ball) error {
	data, err := json.Marshal(ball)
	if err != nil {
		return err
	}

	key := s.key(ball.MatchID)

	// Push to front of list
	if err := s.client.LPush(ctx, key, data).Err(); err != nil {
		return err
	}

	// Keep only last 15 balls
	s.client.LTrim(ctx, key, 0, MaxBalls-1)

	// Reset TTL (match is active)
	s.client.Expire(ctx, key, MatchTTL)

	return nil
}

// UpdateBall replaces a ball entry in the list (for edits).
func (s *RedisStore) UpdateBall(ctx context.Context, ball models.Ball) error {
	return s.PushBall(ctx, ball)
}

func (s *RedisStore) GetLive(ctx context.Context, matchID string) ([]models.Ball, error) {
	key := s.key(matchID)

	results, err := s.client.LRange(ctx, key, 0, MaxBalls-1).Result()
	if err != nil {
		return nil, err
	}

	var balls []models.Ball
	for _, r := range results {
		var ball models.Ball
		if err := json.Unmarshal([]byte(r), &ball); err != nil {
			continue
		}
		balls = append(balls, ball)
	}

	return balls, nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
