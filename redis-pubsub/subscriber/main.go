package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func main()  {
	rbd := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()

	sub := rbd.Subscribe(ctx, "chat")
	fmt.Println("Listening on channel 'chat'...")

	for msg := range sub.Channel() {
		fmt.Printf("[%s] %s\n", msg.Channel, msg.Payload)
	}
}

