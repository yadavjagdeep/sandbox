# redis-pubsub

Simple Redis pub/sub demo in Go. One publisher broadcasts messages, multiple subscribers receive them in real-time.

## Run

```bash
sudo docker-compose up -d
go run subscriber/main.go   # terminal 1
go run subscriber/main.go   # terminal 2 (optional, multiple subscribers)
go run publisher/main.go    # terminal 3, type messages here
```

## Key Concepts

- Pub/sub is fire-and-forget — no message persistence
- If a subscriber is offline, it misses messages permanently
- All subscribers on a channel receive every message (broadcast)
- Unlike message queues (Kafka, RabbitMQ), messages aren't stored or replayed
- Use pub/sub for real-time notifications, chat, live updates
- Use message queues when you need guaranteed delivery and persistence
