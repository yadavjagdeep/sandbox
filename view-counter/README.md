# YouTube View Counter

A distributed view counting system designed to handle 500K+ views/sec with eventual consistency. Demonstrates how platforms like YouTube count video views at massive scale.

## The Problem

A viral video gets 100K+ views per second. You can't just `UPDATE SET views = views + 1` for each view — single row lock contention kills the database at this scale.

## Architecture

```
┌────────────┐     ┌─────┐     ┌───────────────────┐     ┌───────────────┐     ┌────────────────┐
│   Client   │────→│ API │────→│ Kafka (raw-views) │────→│   Validator   │────→│ Kafka           │
│(video player)│   │     │     │ key: video+shard  │     │ + Rule Engine │     │ (valid-views)  │
└────────────┘     └─────┘     └───────────────────┘     └───────────────┘     └───────┬────────┘
                                                                                        │
                                                                         ┌──────────────┼──────────┐
                                                                         ↓              ↓          ↓
                                                                  ┌─────────────┐  Analytics  Monetization
                                                                  │Counter Worker│  (other     (other
                                                                  │ batch +N    │   teams)     teams)
                                                                  └──────┬──────┘
                                                                         ↓
┌────────────┐     ┌───────┐     ┌──────────────┐              ┌─────────────────┐
│   Client   │────→│ Redis │←────│ read-through │←─────────────│   PostgreSQL    │
│  (reader)  │     │ cache │     │  (TTL 1 min) │              │  video_counts   │
└────────────┘     └───────┘     └──────────────┘              └─────────────────┘
```

## Requirements

**Functional:**
- Count views per video
- Filter invalid views via rule engine (black box)
- Serve view count to users

**Non-functional:**
- Scale: 500K views/sec peak
- Read latency: <100ms
- Consistency: eventual (1 minute stale OK)
- Availability: high
- Durability: views can't be lost

**Out of scope:**
- Rule engine internals (fraud/bot detection)
- Video streaming
- User authentication
- Per-user view history
- Analytics (views by region/time)

## Design Decisions

### When Does a View Count?

The video player (client) fires the view event only after the user has watched for 30+ seconds. The server never receives partial watch events. This keeps the server simple — every arriving event has already met the minimum watch threshold.

### Kafka Partitioned by video_id + hash(user_id) % 10

**Problem:** A viral video = all events on one Kafka partition = one consumer bottleneck (hot partition).

**Solution:** Sub-partition with `video_id + hash(user_id) % 10`. Spreads 100K views/sec across 10 partitions (10K/sec each).

**Why user_id hash (not random)?** Same user's events land on same partition — helps the rule engine detect duplicate views within one batch.

### Two Kafka Topics: raw-views → valid-views

- `raw-views`: all incoming events (unvalidated)
- `valid-views`: only events that passed the rule engine (bulk validated)

**Why two topics?** The `valid-views` topic is a fan-out point. Our counter worker is just one consumer. Other teams (analytics, monetization/ad revenue per view) subscribe independently. Rule engine validates once, everyone consumes.

### Rule Engine as Black Box

The validator consumer batches events (100 at a time) and calls the rule engine in bulk. In production, the rule engine checks:
- Watch duration ≥ 30 seconds
- Same user not counting multiple times in a short window
- Bot detection (rate from single IP, behavior patterns)

In our implementation: stub that checks `watch_duration >= 30`.

### Counter Worker Batches Before DB Write

Instead of 100K individual `UPDATE +1`, the consumer aggregates in memory:
- Collects 100 valid events → "video X had 87 views, video Y had 13 views"
- Two DB writes: `UPDATE +87` and `UPDATE +13`
- Reduces DB writes from 100K/sec to a handful per second

### DB Upsert (Atomic Increment, No Explicit Lock)

```sql
INSERT INTO video_counts (video_id, view_count)
VALUES ($1, $2)
ON CONFLICT (video_id)
DO UPDATE SET view_count = video_counts.view_count + $2
```

PostgreSQL handles row-level locking internally during `ON CONFLICT DO UPDATE`. Multiple workers incrementing the same video_id serialize automatically — no explicit `FOR UPDATE` needed. Safe against dirty writes.

### Read-Through Redis Cache (TTL 1 minute)

```
GET /videos/:id/count
  → Redis cache hit (key: views:{video_id}) → return immediately
  → Cache miss → read from DB replica → cache for 1 minute → return
```

- Most reads hit Redis (hot videos always cached)
- 1 minute TTL: count is at most 1 minute stale — acceptable ("1.2M views" vs "1.3M views")
- Add more DB replicas if cache miss rate is high

### Single Counter Table

```sql
video_counts (video_id VARCHAR PRIMARY KEY, view_count BIGINT)
```

One row per video. 800M videos × 16 bytes = ~12GB. Trivially small. Fits in memory on a single PostgreSQL instance.

### Why Not Redis as the Counter (INCR)?

Could work, but adds complexity:
- Redis INCR is fast but volatile (data loss on crash)
- Would need periodic flush to DB for durability
- DB upsert with batching is already fast enough (few writes/sec after aggregation)
- Keeping DB as source of truth is simpler — Redis is just a read cache

## Back-of-Envelope

```
DAU:        1 billion
Videos/day: 10 per user
Views/day:  10 billion
Views/sec:  ~115K average
Peak (5x):  ~575K views/sec
Event size: ~200 bytes
Kafka throughput: 575K × 200B = ~115 MB/sec
Counter table: 800M rows × 16 bytes = ~12 GB
```

## API

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/views` | Record a view event (from video player) |
| GET | `/videos/:id/count` | Get view count for a video |

**POST /views**
```json
{"video_id": "dQw4w9WgXcQ", "user_id": "user_123", "watch_duration": 45}
```

**GET /videos/:id/count**
```json
{"video_id": "dQw4w9WgXcQ", "view_count": 1234567}
```

## Data Model

```sql
CREATE TABLE video_counts (
    video_id VARCHAR(50) PRIMARY KEY,
    view_count BIGINT NOT NULL DEFAULT 0
);
```

That's it. One table, one row per video, one counter.

Raw events live in Kafka topics (retention-based). No event-level table needed for counting.

## Running Locally

### Start Infrastructure

```bash
sudo docker-compose up -d
```

Starts PostgreSQL, Redis, Kafka + Zookeeper.

### Run the API

```bash
go run .
```

### Run the Validator

```bash
go run ./validator/
```

### Run the Counter Worker

```bash
go run ./counter/
```

### Test: Record Views

```bash
# Single view (simulates player after 30s watch)
curl -X POST http://localhost:8080/views \
  -H "Content-Type: application/json" \
  -d '{"video_id": "dQw4w9WgXcQ", "user_id": "user_123", "watch_duration": 45}'

# Response: {"status": "accepted"}
```

### Test: Burst of Views

```bash
# 200 views from different users
for i in $(seq 1 200); do
  curl -s -X POST http://localhost:8080/views \
    -H "Content-Type: application/json" \
    -d "{\"video_id\": \"dQw4w9WgXcQ\", \"user_id\": \"user_$i\", \"watch_duration\": 45}" &
done
wait
```

### Test: Get Count

```bash
# Wait for pipeline to process
sleep 5

curl http://localhost:8080/videos/dQw4w9WgXcQ/count
# Response: {"video_id": "dQw4w9WgXcQ", "view_count": 200}
```

### Test: Invalid Views (filtered by rule engine)

```bash
# Watch duration < 30s — should NOT count
curl -X POST http://localhost:8080/views \
  -H "Content-Type: application/json" \
  -d '{"video_id": "dQw4w9WgXcQ", "user_id": "bot_999", "watch_duration": 5}'
```

## Project Structure

```
view-counter/
├── main.go              # API (POST /views, GET /videos/:id/count)
├── validator/main.go    # raw-views → rule engine → valid-views
├── counter/main.go      # valid-views → batch aggregate → DB increment
├── models/views.go      # ViewEvent, VideoCount
├── rule/engine.go       # Rule engine stub (black box)
├── store/postgres.go    # DB: upsert increment, get count
├── store/cache.go       # Redis read-through cache (TTL 1 min)
├── migration/schema.sql # video_counts table
├── docker-compose.yaml  # PostgreSQL + Redis + Kafka
└── README.md
```

## Cleanup

```bash
sudo docker-compose down -v
```
