package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rbd        *redis.Client
	rate       float64
	bucketSize int64
}

func New(rbd *redis.Client, rate float64, bucketSize int64) *RateLimiter {
	return &RateLimiter{
		rbd:        rbd,
		rate:       rate,
		bucketSize: bucketSize,
	}
}

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// Lua script runs atomically in Redis - no race condition
	script := redis.NewScript(`
			local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local bucket_size = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])

		local data = redis.call("HMGET", key, "tokens", "last_refill")
		local tokens = tonumber(data[1])
		local last_refill = tonumber(data[2])

		if tokens == nil then
			tokens = bucket_size
			last_refill = now
		end

		local elapsed = now - last_refill
		local new_tokens = elapsed * rate
		tokens = math.min(bucket_size, tokens + new_tokens)
		last_refill = now

		if tokens >= 1 then
			tokens = tokens - 1
			redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)
			redis.call("EXPIRE", key, 60)
			return 1
		else
			redis.call("HMSET", key, "tokens", tokens, "last_refill", last_refill)
			redis.call("EXPIRE", key, 60)
			return 0
		end
	`)

	now := float64(time.Now().UnixMilli()) / 1000.0
	result, err := script.Run(ctx, rl.rbd, []string{key}, rl.rate, rl.bucketSize, now).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}
