# Recent Searches

Cross-device recent search history — showing the last 10 unique searches per user. Designed for high-scale: sub-millisecond reads, high throughput writes, with an async analytics pipeline.

## The Problem

A search app (think Google, Amazon) needs to show a user's recent searches instantly when they open the search box. Requirements:
- **Fast:** user clicks within 5 seconds of opening the app — no time for slow DB queries
- **Cross-device:** search on phone, see it on laptop
- **Dedup:** searching the same thing twice doesn't create duplicates — it moves to the top
- **Bounded:** only last 10 searches shown
- **Analytics:** every search stored for business decisions (trending queries, personalization)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Read Path (sub-millisecond)                                     │
│                                                                 │
│  GET /searches/:userId → Redis ZREVRANGE → return top 10        │
│  Redis miss (TTL 30 days expired) → return empty []             │
│  No backfill. Old searches lose relevance over time.            │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Write Path                                                      │
│                                                                 │
│  POST /searches {userId, query}                                 │
│    → Redis ZADD (instant, dedup, bounded to 10, resets TTL)     │
│    → Kafka produce (async, non-blocking)                        │
│        → Consumer batches 25 records                            │
│            → DynamoDB BatchWriteItem (with 6-month TTL)         │
│                → CDC (DynamoDB Streams) → S3 (archive)          │
│                    → Athena (analytics queries)                  │
└─────────────────────────────────────────────────────────────────┘
```

## Design Decisions

### Why Redis (not SQL)?

Users expect results within 5 seconds of opening the app. SQL with indexes:
- Adds latency (network + disk seek + index traversal)
- Indexes slow down writes under heavy load (every search = a write)
- At Google scale, the write pressure on indexes degrades read performance

Redis sorted sets:
- Sub-millisecond reads (in-memory)
- `ZADD` is O(log 10) ≈ O(1) — no index maintenance
- Reads and writes don't block each other

### Why Redis sorted set (not a list)?

Dedup. If user searches "kafka" again, it should move to the top — not appear twice.

With a list (`LPUSH`):
- Need `LREM` first (O(n) scan) then `LPUSH` — two operations

With sorted set (`ZADD`):
- `ZADD` with same member + new score → automatically updates (dedup + move to top in one atomic op)

```
ZADD searches:user123 1718950800000 "kafka"     ← first search
ZADD searches:user123 1718951400000 "kafka"     ← same query, new timestamp → moves to top
ZREMRANGEBYRANK searches:user123 0 -11          ← keep only top 10
```

### Why no backfill from DynamoDB?

If a user hasn't searched in 30 days (Redis TTL expires), their old searches are irrelevant. Relevance decays with time. A fresh start is better UX than showing month-old queries.

Read path is **purely Redis**. DynamoDB is never hit on the user-facing path.

### Why Kafka between API and DynamoDB?

1. **Latency:** API returns immediately after Redis write. Kafka produce is async (goroutine).
2. **Throughput:** Searches happen massively. Kafka absorbs spikes (append-only log, like Bitcask).
3. **Batching:** Consumer collects 25 records before flushing to DynamoDB. Reduces write operations (and cost) by 25x.

### Why DynamoDB (not SQL) for persistence?

- No indexes needed (partition key + sort key gives ordering for free)
- Native TTL (auto-delete after 6 months, no cleanup cron)
- DynamoDB Streams for CDC → S3 archival
- Pay-per-request: zero cost when idle
- Each search = a new record (unbounded history, no dedup — analytics needs everything)

### DynamoDB Schema

```
Table: user_searches
Partition key: user_id (String)
Sort key: searched_at (Number, Unix milliseconds)

{
  "user_id": "user_123",
  "searched_at": 1718950800000,
  "query": "bloom filter",
  "ttl": 1734502800          ← 6 months from now (Unix seconds)
}
```

### Analytics (not implemented — called out here)

- DynamoDB TTL auto-deletes records after 6 months
- DynamoDB Streams (CDC) captures deletions → Lambda writes to S3 before expiry
- S3 holds permanent archive (all searches ever)
- Athena queries S3 for business analytics (trending searches, user behavior, etc.)

### Redis TTL = 30 days

- Every write resets the TTL (active users stay warm)
- Inactive users auto-evict after 30 days
- No manual cleanup needed

## Storage Layers

| Layer | Purpose | User-facing? | Duration |
|-------|---------|-------------|----------|
| Redis | Serve last 10 searches | Yes (only read path) | 30 days (auto-evict inactive) |
| Kafka | Buffer writes, absorb spikes | No | Transient |
| DynamoDB | Persist all searches for analytics | No | 6 months (TTL) |
| S3 | Permanent archive | No | Forever |
| Athena | Query historical data | No | On-demand |

## Running Locally

### Prerequisites
- Docker + docker-compose
- Go 1.22+
- Redis running on localhost:6379 (or use the Docker one)

### Step 1: Start Infrastructure

```bash
sudo docker-compose up -d
```

Starts:
- Redis on port 6379
- Kafka + Zookeeper on port 9092
- DynamoDB Local on port 8000
- DynamoDB Admin UI on port 8001

### Step 2: Create DynamoDB Table

```bash
AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local \
aws dynamodb create-table \
  --endpoint-url http://localhost:8000 \
  --table-name user_searches \
  --attribute-definitions \
    AttributeName=user_id,AttributeType=S \
    AttributeName=searched_at,AttributeType=N \
  --key-schema \
    AttributeName=user_id,KeyType=HASH \
    AttributeName=searched_at,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### Step 3: Start the Consumer

```bash
go run ./consumer/
```

### Step 4: Start the API Server

```bash
go run .
```

### Step 5: Test

```bash
# Add searches
curl -X POST http://localhost:8080/searches \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123", "query": "bloom filter"}'

curl -X POST http://localhost:8080/searches \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123", "query": "bitcask"}'

curl -X POST http://localhost:8080/searches \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123", "query": "kafka"}'

# Get recent searches (most recent first)
curl http://localhost:8080/searches/user_123
```

Response:
```json
{
  "user_id": "user_123",
  "searches": ["kafka", "bitcask", "bloom filter"]
}
```

### Test Dedup

```bash
# Search "bloom filter" again — moves to top, no duplicate
curl -X POST http://localhost:8080/searches \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123", "query": "bloom filter"}'

curl http://localhost:8080/searches/user_123
```

Response:
```json
{
  "user_id": "user_123",
  "searches": ["bloom filter", "kafka", "bitcask"]
}
```

### Test 10-Item Cap

```bash
# Add 12 more searches — oldest ones get evicted
for i in $(seq 1 12); do
  curl -s -X POST http://localhost:8080/searches \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"user_123\", \"query\": \"query_$i\"}"
done

curl http://localhost:8080/searches/user_123
# Returns exactly 10 items, newest first
```

### Verify DynamoDB Persistence

After sending 25+ searches, the consumer flushes to DynamoDB:
```
Consumer started. Waiting for messages...
Flushed 25 records to DynamoDB
```

View records in DynamoDB Admin UI: http://localhost:8001

Or via CLI:
```bash
AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local \
aws dynamodb scan \
  --endpoint-url http://localhost:8000 \
  --table-name user_searches \
  --max-items 5 \
  --region us-east-1
```

### Test Unknown User

```bash
curl http://localhost:8080/searches/unknown_user
```

Response:
```json
{
  "user_id": "unknown_user",
  "searches": []
}
```

## Project Structure

```
recent-searches/
├── main.go              # API server (Echo)
├── handler/search.go    # POST /searches + GET /searches/:userId
├── store/redis.go       # Redis sorted set: ZADD, ZREVRANGE, TTL
├── consumer/main.go     # Kafka consumer → DynamoDB batch writer
├── docker-compose.yaml  # Redis + Kafka + DynamoDB + DynamoDB Admin UI
├── go.mod
└── README.md
```

## Cleanup

```bash
sudo docker-compose down -v
```
