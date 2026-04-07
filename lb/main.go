package main

import (
	"log"
	"time"
)

func main() {

	backends := []string{
		"localhost:8080",
		"localhost:8081",
		"localhost:8082",
	}

	lb := NewLoadBalancer(backends)

	go  lb.HealthCheck(10 *time.Second) // check every 10 seconds

	if err := lb.Start(":9090"); err != nil {
		log.Fatalf("failed to start: %v", err)
	}

}
