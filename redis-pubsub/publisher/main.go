package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)


func main()  {
	rbd := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()

	fmt.Println("Type messages to publish (ctrl+c to quit):")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		rbd.Publish(ctx, "chat", msg)
		fmt.Printf("published: %s\n", msg)
	}
}
