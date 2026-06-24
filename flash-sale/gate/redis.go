package gate

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisGate struct {
	client *redis.Client
}

func NewRedisGate(addr string) *RedisGate {
	return &RedisGate{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
	}
}

func (g *RedisGate) key(itemID string) string {
	return "gate:" + itemID
}

// set slot cont for an item's flash sale
func (g *RedisGate) InitGate(ctx context.Context, itemID string, slots int64) error {
	return g.client.Set(ctx, g.key(itemID), slots, 0).Err()
}

func (g *RedisGate) TryEnter(ctx context.Context, itemID string) (bool, error) {
	val, err := g.client.Decr(ctx, g.key(itemID)).Result()
	if err != nil {
		return false, err
	}

	if val < 0 {
		g.client.Incr(ctx, g.key(itemID))
		return false, nil
	}
	return true, nil
}

func (g *RedisGate) Release(ctx context.Context, itemID string) error {
	return g.client.Incr(ctx, g.key(itemID)).Err()
}

func (g *RedisGate) GetRemaining(ctx context.Context, itemID string) (int64, error) {
	val, err := g.client.Get(ctx, g.key(itemID)).Int64()
	if err != nil {
		return 0, nil
	}
	return max(val, 0), nil
}

func (g *RedisGate) Ping(ctx context.Context) error {
	return g.client.Ping(ctx).Err()
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (g *RedisGate) Info(ctx context.Context, itemID string) string {
	val, _ := g.client.Get(ctx, g.key(itemID)).Int64()
	return fmt.Sprintf("gate:%s = %d slots remaining", itemID, val)
}
