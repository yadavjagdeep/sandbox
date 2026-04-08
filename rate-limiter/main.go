package main

import (
	"context"
	"fmt"
	"net/http"
	"rate-limiter/limiter"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	rbd := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	rl := limiter.New(rbd, 5, 10)

	e := echo.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := "rl:" + c.RealIP()

			allowed, err := rl.Allow(context.Background(), key)

			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "rate limiter error"})
			}

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			}
			return next(c)
		}
	})

	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"messgae": "PONG"})
	})

	fmt.Println("server on :8080 (5 req/sec, burst 10)")
	e.Logger.Fatal(e.Start(":8080"))
}
