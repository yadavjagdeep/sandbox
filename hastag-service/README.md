# Hashtag Service

A hashtag aggregation service that processes high-volume photo tagging events, maintains per-hashtag photo counts and top 100 photos, and serves them via a fast API for a dashboard.

## Problem Statement

A data science team sends events every time a user posts a photo with a hashtag. Each event contains the hashtag, photo info, and the current top 100 photos for that hashtag. We need to:

- Count total photos per hashtag (incrementally)
- Store the latest top 100 photos per hashtag
- Serve this data via API with fast response times
- Handle millions of events per second

## Architecture

```
Data Science Team
       │
       ▼
┌─────────────────────┐
│  Kafka Topic         │
│  "posts-by-user"     │  Partitioned by userId
│  (partition 0,1,2)   │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Adapter             │  Reads from posts-by-user
│  (re-partitioner)    │  Writes to posts-by-hashtag
│                      │  Changes partition key: userId → hashtag
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Kafka Topic         │
│  "posts-by-hashtag"  │  Partitioned by hashtag
│  (partition 0,1,2)   │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Consumer            │  Each hashtag owned by exactly one consumer
│  - In-memory count   │  No race conditions on counts
│  - In-memory top 100 │  Flushes to Postgres every 30 sec
└──────────┬──────────┘
           │ (flush every 30s)
           ▼
┌─────────────────────┐
│  Postgres            │  Durable storage
│  hashtags table      │  Single table, JSONB for top 100
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  API Server (:8080)  │  GET /hastag/:name
│  Reads from Postgres │  Returns count + top 100
└─────────────────────┘
```

## Why Re-Partition by Hashtag?

The incoming Kafka topic is partitioned by userId (data science team's choice). This means:

```
Without re-partitioning:
  Partition 0: user1 posts #sunset → Consumer A increments #sunset
  Partition 1: user2 posts #sunset → Consumer B increments #sunset
  Partition 2: user3 posts #sunset → Consumer C increments #sunset

  Problem: 3 consumers updating #sunset concurrently
  Need shared state (Redis) or locking to coordinate
```

```
With re-partitioning:
  Partition 0: ALL #sunset messages → Consumer A (sole owner)
  Partition 1: ALL #travel messages → Consumer B (sole owner)
  Partition 2: ALL #food messages   → Consumer C (sole owner)

  No shared state, no race conditions, simple count++
```

The adapter is a lightweight consumer that reads from one topic and writes to another with a different partition key. It solves the data ownership problem.

## Why In-Memory Counts + Periodic Flush?

At millions of events per second, writing to Postgres on every event would kill the database. Instead:

- Count in memory (fast, no I/O)
- Flush to Postgres every 30 seconds (batched writes)
- Postgres handles ~1 write per hashtag per 30 seconds instead of thousands per second

Tradeoff: if the consumer crashes, you lose up to 30 seconds of counts. Acceptable for a dashboard showing approximate counts like "1.2M photos."

## Why Single Table with JSONB?

```sql
hashtags (
    name TEXT PRIMARY KEY,
    photo_count BIGINT DEFAULT 0,
    top_photos JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMP DEFAULT NOW()
)
```

- One query, one row fetch per hashtag — no JOINs
- UPSERT is a single atomic write
- JSONB stores the top 100 photos as an array — read all 100 together, always
- Alternative (separate photos table) would need a JOIN and 100 rows per hashtag

## Kafka Message Format

```json
{
    "user_id": 123,
    "hashtag": "#sunset",
    "photo_id": "photo_45678",
    "top_100_posts": [
        {"url": "https://photos.example.com/photo_123.jpg", "likes": 5000},
        {"url": "https://photos.example.com/photo_456.jpg", "likes": 4200},
        ...
    ]
}
```

Partition key for posts-by-user: userId
Partition key for posts-by-hashtag: hashtag

## Setup

```bash
# Start Kafka + Postgres
sudo docker-compose up -d

# Wait 15 seconds for Kafka to start, then create topics
sudo docker exec -it hashtag-kafka /opt/kafka/bin/kafka-topics.sh \
  --create --topic posts-by-user --partitions 3 --replication-factor 1 \
  --bootstrap-server kafka:29092

sudo docker exec -it hashtag-kafka /opt/kafka/bin/kafka-topics.sh \
  --create --topic posts-by-hashtag --partitions 3 --replication-factor 1 \
  --bootstrap-server kafka:29092
```

## Run (4 terminals)

```bash
# Terminal 1: Adapter (re-partitions userId → hashtag)
go run adapter/main.go

# Terminal 2: Consumer (processes events, flushes to Postgres)
go run consumer/main.go

# Terminal 3: API server
go run main.go

# Terminal 4: Producer (simulates data science events)
go run producer/main.go
```

## Test

After producer finishes and consumer flushes (30 sec or Ctrl+C consumer):

```bash
curl http://localhost:8080/hastag/sunset
curl http://localhost:8080/hastag/travel
curl http://localhost:8080/hastag/food
curl http://localhost:8080/hastag/coding
curl http://localhost:8080/hastag/music
```

## Scaling Considerations

Bottlenecks we identified and solved:
- High write volume → in-memory counting with periodic flush (not per-event DB writes)
- Concurrent hashtag updates → re-partition by hashtag (single owner per hashtag)
- Single table design → JSONB avoids JOINs, one row per hashtag

Further scaling (not implemented):
- Multiple consumer instances in a consumer group (Kafka handles partition assignment)
- Redis cache in front of Postgres for hot hashtags
- CDN cache for the API responses of trending hashtags
- Batch flush only dirty hashtags (skip unchanged ones)

## Components

```
hastag-service/
├── docker-compose.yaml    # Kafka (Apache, KRaft mode) + Postgres
├── main.go                # API server — reads from Postgres
├── adapter/main.go        # Re-partitions: posts-by-user → posts-by-hashtag
├── consumer/main.go       # Processes events, maintains counts, flushes to Postgres
├── producer/main.go       # Simulates data science team sending events
└── models/hastag.go       # Data structures
```
