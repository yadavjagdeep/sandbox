# rate-limiter

Token bucket rate limiter in Go using Redis and Echo middleware.

## Run

```bash
# Start Redis (skip if already running locally)
sudo docker-compose up -d

go run main.go
```

## Test

```bash
# Single request
curl http://localhost:8080/ping

# Burst — will get 429 after ~10 requests
for i in $(seq 1 15); do curl -s http://localhost:8080/ping; echo; done
```

## How It Works

Token bucket algorithm with Redis:
- Bucket holds max 10 tokens (burst size)
- Refills at 5 tokens/second (sustained rate)
- Each request costs 1 token
- Empty bucket → 429 Too Many Requests

Tokens are calculated lazily — no background process. On each request, elapsed time since last request is used to calculate how many tokens to add.

State stored in Redis as a hash per client IP:
```
Key: "rl:<client_ip>"
  tokens      → current token count
  last_refill → timestamp of last calculation
```

Lua script ensures atomic read-calculate-write in Redis — safe under concurrent requests.

## Rate Limiting Algorithms

- Fixed Window: simple counter per time window, burst problem at boundaries
- Sliding Window Log: store every timestamp, accurate but memory heavy
- Sliding Window Counter: weighted average of two windows, good balance
- Token Bucket: allows bursts up to bucket size, smooth average rate (this project)
- Leaky Bucket: fixed processing rate, no bursts, smoothest output

## Where Rate Limiters Live

- API Gateway / LB: coarse limits before traffic hits app servers
- App middleware: per-user, per-endpoint fine-grained limits (this project)
- Dedicated service: centralized rate limiting for microservices
- Multiple layers: production systems often rate limit at more than one level
