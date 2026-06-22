# Live Commentary

Real-time cricket commentary system like Cricbuzz — ball-by-ball text updates served to millions of users with a cost-efficient architecture.

## The Problem

During a live cricket match:
- A commentator types ball-by-ball updates
- Millions of users (mostly in India) open the app and expect to see live commentary instantly
- Users click within 5 seconds of opening — no time for slow DB queries
- The commentary must be cross-device consistent

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│  Write Path (Commentator Panel)                                         │
│                                                                        │
│  POST /commentary {ball data + text}                                   │
│    → Write to Redis (LPUSH, keep last 15 balls)                        │
│    → Write to PostgreSQL (INSERT, full history)                        │
│    → Return status of BOTH to commentator UI                           │
│    → If either fails → show red icon, commentator retries              │
└────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────┐
│  Read Path (10M Users)                                                  │
│                                                                        │
│  Poll every 5s: GET /commentary/:matchId/live                          │
│    → Nginx cache (5s TTL) → serves from memory                         │
│    → Cache miss → Go server → Redis LRANGE → return 15 balls           │
│                                                                        │
│  "Load More": GET /commentary/:matchId/history?cursor=X                │
│    → PostgreSQL replica → cursor-based pagination                      │
└────────────────────────────────────────────────────────────────────────┘
```

## Design Decisions

### Why Redis for the Live Path?

10M users polling every 5 seconds = 2M requests/sec.

- **PostgreSQL can't handle that** — even with replicas and indexes, the connection overhead + disk I/O would require 50+ servers.
- **Redis serves from memory** — sub-millisecond reads, 100K+ ops/sec per node.
- Data per match is ~3KB (15 balls × ~200 bytes). Trivially small.

### Why Not WebSocket/SSE?

Commentary updates once every ~30 seconds (one ball). That's low frequency.

- WebSocket: 10M persistent connections = massive server fleet, expensive state management
- Short polling + cache: stateless, cheap, max 5 seconds staleness — perfectly acceptable for text commentary

### Handling 10M Users (Nginx Cache in Production)

All 10M users request the **exact same data** (same match, same last 15 balls). It's a broadcast, not personalized content.

```nginx
proxy_cache_path /tmp/cache levels=1:2 keys_zone=commentary:10m max_size=100m;

location /commentary/ {
    proxy_cache commentary;
    proxy_cache_valid 200 5s;
    proxy_pass http://go-backend;
}
```

- 2M requests/sec hit Nginx → all served from in-memory cache
- Go server gets hit **once every 5 seconds per match** (when cache expires)
- 4-5 Nginx nodes handle all traffic vs 50 Go servers without cache

**This is not a CDN.** Users are in India, servers are in India. It's just a reverse proxy cache to absorb the broadcast traffic.

### Why Not CDN?

Most users are in India. Latency between an Indian user and an Indian server is already ~10-20ms. CDN's geographic distribution doesn't add value here. Nginx proxy cache in the same region achieves the same effect.

### Storage: PostgreSQL (Master + 2 Replicas)

- **Master:** handles writes (one insert per ball, ~once every 30 seconds per match — trivial)
- **Replicas:** handle "Load More" queries (cursor-based pagination — low volume, only users who scroll)

Data per match is ~60KB (300 balls × 200 bytes). PostgreSQL handles this effortlessly.

### Write Pattern: No Queues, No CDC, No Async

Writes happen once every 30 seconds. That's 2 writes per minute per match. Even 10 concurrent matches = 20 writes/min. There's no need for Kafka/CDC for this volume.

```
Commentator submits ball →
  Write to Redis (live users)
  Write to PostgreSQL (history)
  If either fails → show error to commentator → they retry
```

Both writes are in the same API call. Failures are infra issues (rare) — the commentator (human in the loop) handles retries manually.

### Redis TTL

- Match key expires after 6 hours of no writes
- No manual cleanup needed
- Only active matches consume Redis memory

### Commentator Panel

The commentator sees the status of each write:

```
Ball 4.3 submitted:
  Redis:      ✅ success
  PostgreSQL: ✅ success

Ball 4.4 submitted:
  Redis:      ✅ success
  PostgreSQL: 🔴 failed [Retry]
```

If PostgreSQL fails but Redis succeeds — live users still see the ball. Commentator retries the DB write when ready.

### Pagination for "Load More"

Cursor-based (not offset-based):

```
GET /history?cursor=27&limit=20
→ Returns balls with ball_number < 27, ordered descending, limit 20
→ Response includes next_cursor for the next page
```

Efficient — no OFFSET scanning. Works well with the index on `(match_id, ball_number DESC)`.

## Data Model

```json
{
  "match_id": "ipl_2024_final",
  "ball_number": 27,
  "over_number": "4.3",
  "bowler": "Bumrah",
  "batsman": "Kohli",
  "runs": 4,
  "is_wicket": false,
  "is_boundary": true,
  "text": "FOUR! Driven through covers. Short of length, Kohli punched it through the gap.",
  "score": "45/1"
}
```

## API Endpoints

| Method | Endpoint | Purpose | Served by |
|--------|----------|---------|-----------|
| POST | `/commentary` | Submit new ball | Redis + PostgreSQL |
| PUT | `/commentary` | Edit existing ball | Redis + PostgreSQL |
| GET | `/commentary/:matchId/live` | Last 15 balls (polled every 5s) | Redis |
| GET | `/commentary/:matchId/history?cursor=X&limit=N` | Older balls (on "Load More") | PostgreSQL |

## Cost Breakdown (Production, 10M users)

| Component | Count | Role |
|-----------|-------|------|
| Nginx nodes | 4-5 | Cache + serve all user traffic |
| Go servers | 2-3 | Handle cache misses (once per 5s per match) + writes |
| Redis | 1 node | Store last 15 balls per active match |
| PostgreSQL master | 1 | Writes |
| PostgreSQL replicas | 2 | "Load More" reads |

Without Nginx cache: ~50 Go servers. With cache: 4-5 Nginx + 2-3 Go pods. Massive cost savings.

## Running Locally

### Start Infrastructure

```bash
sudo docker-compose up -d
```

Starts PostgreSQL (with schema auto-applied) and Redis.

### Run the Server

```bash
go run .
```

### Submit Commentary

```bash
curl -X POST http://localhost:8080/commentary \
  -H "Content-Type: application/json" \
  -d '{
    "match_id": "ipl_2024_final",
    "ball_number": 1,
    "over_number": "0.1",
    "bowler": "Bumrah",
    "batsman": "Rohit",
    "runs": 0,
    "is_wicket": false,
    "is_boundary": false,
    "text": "Dot ball. Good length, defended back to the bowler.",
    "score": "0/0"
  }'
```

Response:
```json
{"ball_number": 1, "redis": "success", "postgres": "success"}
```

### Get Live Commentary

```bash
curl http://localhost:8080/commentary/ipl_2024_final/live
```

Response:
```json
{
  "match_id": "ipl_2024_final",
  "count": 4,
  "balls": [
    {"ball_number": 4, "text": "Single to fine leg...", "score": "5/1"},
    {"ball_number": 3, "text": "WICKET! Caught behind...", "score": "4/1"},
    {"ball_number": 2, "text": "FOUR! Short and wide...", "score": "4/0"},
    {"ball_number": 1, "text": "Dot ball...", "score": "0/0"}
  ]
}
```

### Load More (Paginated History)

```bash
curl "http://localhost:8080/commentary/ipl_2024_final/history?cursor=3&limit=2"
```

Returns balls older than ball_number 3, paginated.

### Edit a Ball

```bash
curl -X PUT http://localhost:8080/commentary \
  -H "Content-Type: application/json" \
  -d '{
    "match_id": "ipl_2024_final",
    "ball_number": 2,
    "text": "FOUR! Updated commentary with more detail.",
    "runs": 4, "is_boundary": true,
    "bowler": "Bumrah", "batsman": "Rohit",
    "over_number": "0.2", "score": "4/0"
  }'
```

## Project Structure

```
live-commentry/
├── main.go                  # API server (Echo)
├── handler/commentry.go     # POST/PUT (commentator) + GET (users)
├── store/redis.go           # Redis list: push ball, get live 15
├── store/postgres.go        # PostgreSQL: insert, update, paginated read
├── model/balls.go           # Ball data model
├── migration/schema.sql     # Table + index
├── docker-compose.yaml      # PostgreSQL + Redis
├── go.mod
└── README.md
```

## Cleanup

```bash
sudo docker-compose down -v
```
