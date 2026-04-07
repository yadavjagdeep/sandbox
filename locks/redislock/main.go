package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis distributed lock using SET NX (Set if Not eXists)
// Run this in TWO terminals to see it work — same as process lock but via Redis.

const lockKey = "myapp:lock"
const lockTTL = 15 * time.Second // Auto-expire lock after 15s (safety net if process crashes)


func main() {
	ctx := context.Background()
	pid := os.Getpid()

	// connect to local redis

	rbd := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})

	defer rbd.Close()

	if err := rbd.Ping(ctx).Err(); err != nil {
		fmt.Println("cannot connect to redis:", err)
		fmt.Println("make sure redis is running: redis-server or docker run -p 6379:6379 redis")
		return
	}

	fmt.Printf("[PID %d] Trying to acquire lock...\n", pid)

	lockValue := fmt.Sprintf("process-%d", pid)

	for {
		err := rbd.SetArgs(ctx, lockKey, lockValue, redis.SetArgs{
			Mode: "NX",
			TTL:  lockTTL,
		}).Err()
		if err == redis.Nil {
			fmt.Printf("[PID %d] Lock is held by someone else, retrying...\n", pid)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err != nil {
			fmt.Println("Redis err: ", err)
			return
		}

		break // we got the lock!
	}

	fmt.Printf("[PID %d] Lock acquired! Working for 10 seconds...\n", pid)

	for i := 1; i <= 10; i++ {
		fmt.Printf("[PID %d] working... %d/10\n", pid, i)
		time.Sleep(1 * time.Second)
	}

	// Release lock — but ONLY if we still own it (check value matches)
	// This prevents releasing someone else's lock if ours expired

	val, err := rbd.Get(ctx, lockKey).Result()
	if err == nil && val == lockValue {
		rbd.Del(ctx, lockKey)
		fmt.Printf("[PID %d] Lock released!\n", pid)
	} else {
		fmt.Printf("[PID %d] Lock already expired or taken by someone else\n", pid)
	}
}
