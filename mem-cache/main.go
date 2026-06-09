package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	port := flag.String("port", "7070", "server port")
	maxMB := flag.Int64("max-mb", 256, "max memory in MB")
	join := flag.String("join", "", "seed node address to join (e.g., localhost:7071)")
	flag.Parse()

	maxBytes := *maxMB * 1024 * 1024
	selfID := "localhost:" + *port

	fmt.Printf("starting mem-cache: port=%s, max=%dMB, self=%s\n", *port, *maxMB, selfID)

	ring := NewRing(128)
	cache := NewCache(maxBytes)
	cluster := NewCluster(selfID, ring)

	// join existing cluster if --join is specified
	if *join != "" {
		if err := cluster.Join(*join); err != nil {
			fmt.Fprintf(os.Stderr, "failed to join cluster: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("joined cluster via %s, peers: %v\n", *join, cluster.peers)
	}

	// start heartbeat
	cluster.StartHeartbeat()

	server := NewServer(*port, cache, ring, cluster, selfID)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
