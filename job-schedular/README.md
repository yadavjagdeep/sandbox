# Distributed Job Scheduler

A distributed job scheduler that guarantees execution within a 30-second SLA. Supports one-time scheduled tasks and recurring events. Designed with separation of concerns — pollers, queue, and executors scale independently.

## Architecture

```
┌──────────┐       ┌──────────────┐       ┌─────────────┐       ┌──────────────┐
│  Client  │─POST─→│   Web API    │─INSERT→│ Jobs Table  │←─POLL─│   Pollers    │
└──────────┘       └──────────────┘       │ (PostgreSQL)│       │  (2-3 pods)  │
                                          └─────────────┘       └──────┬───────┘
                                                                       │ push
                                                                       ↓
                                                                ┌─────────────┐
                                                                │  SQS Queue  │
                                                                └──────┬──────┘
                                                                       │ pull
                                                                       ↓
                                                                ┌──────────────┐
                                                                │  Executors   │
                                                                │  (N pods)    │
                                                                └──────┬───────┘
                                                                       │ completion
                                                                       ↓
                                                                ┌─────────────────┐
                                                                │ Completion Queue │
                                                                └─────────────────┘
```

## Requirements

- One-time scheduled tasks executed at a fixed time
- Recurring events (cron-like)
- 30-second SLA (job fires within 30s of scheduled time)
- Minute-level granularity
- No retries on failure

## Design Decisions

### Separation of Concerns

| Component | Role | Talks to |
|-----------|------|----------|
| Web API | Accept scheduling requests | PostgreSQL |
| Pollers (2-3 pods) | Pick due jobs, push to queue, replenish recurring | PostgreSQL + SQS |
| Executors (N pods) | Pull from queue, execute | SQS only |
| Completion consumer | Batch update timestamps | SQS + PostgreSQL |

### Why Separate Pollers from Executors?

Million executors polling PostgreSQL directly = million DB connections. PostgreSQL limit is ~500. The DB becomes the bottleneck.

With separation:
- Only 2-3 pollers talk to the DB (few connections)
- SQS handles fan-out to N executors
- Executors scale independently based on queue depth

### Job Pickup: FOR UPDATE SKIP LOCKED

Multiple pollers grab jobs without duplicates:

```sql
SELECT * FROM jobs
WHERE scheduled_at <= NOW() + INTERVAL '30 seconds'
AND picked_at IS NULL
ORDER BY scheduled_at
LIMIT 100
FOR UPDATE SKIP LOCKED;
```

- `FOR UPDATE`: locks selected rows atomically
- `SKIP LOCKED`: if another poller locked a row, skip it (no waiting)
- Each poller gets a unique batch — no contention, no duplicates

### Partial Index

```sql
CREATE INDEX idx_jobs_pending ON jobs(scheduled_at) WHERE picked_at IS NULL;
```

Only indexes unpicked jobs. As jobs get picked, they drop out of the index automatically. Index stays small regardless of total historical data.

### No Status Column

State is derived from timestamps:

| picked_at | started_at | completed_at | State |
|-----------|------------|--------------|-------|
| NULL | NULL | NULL | Scheduled (waiting) |
| set | NULL | NULL | Picked (in queue) |
| set | set | NULL | Executing |
| set | set | set | Done |

No redundant column. No risk of status being out of sync.

### Recurring Jobs: Pre-insert Next 5 Schedules

For recurring jobs, 5 future executions are always present in the jobs table:
- Poller checks: if pending count < 5 → insert next schedules
- If one execution is missed, the next ones still fire
- Cancel = `DELETE WHERE recurring_job_id = X AND picked_at IS NULL`

### Completion Updates via Queue

Executors don't talk to the DB. They push completion events (`started_at`, `completed_at`) to a separate queue. A lightweight consumer batches and updates the DB. Not time-sensitive — eventual consistency is fine for observability.

### Timestamps for Forecasting

```
picked_at - scheduled_at  = queue wait time
started_at - picked_at    = SQS delivery time
completed_at - started_at = execution duration
```

These metrics enable predictive autoscaling: look at upcoming job count + average execution time → scale workers proactively before the SLA is breached.

## Data Model

```sql
CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    command JSONB NOT NULL,
    scheduled_at TIMESTAMP NOT NULL,
    picked_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    recurring_job_id BIGINT
);

CREATE TABLE recurring_jobs (
    id BIGSERIAL PRIMARY KEY,
    command JSONB NOT NULL,
    cron_expr VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
```

## What's NOT Implemented (Production Considerations)

| Feature | Description |
|---------|-------------|
| Predictive autoscaling | Scale workers based on upcoming job count, not reactive lag |
| Cron expression parsing | Parse cron strings to compute exact next execution times |
| Completion consumer | Separate service to batch-update started_at/completed_at in DB |
| Dead job detection | Detect jobs stuck in "picked" state (worker crashed) |
| Job cancellation API | Cancel pending one-time or recurring jobs |
| Metrics/alerting | Queue depth, SLA breaches, P99 execution latency |
| Rate limiting | Prevent scheduling too many jobs for the same time |

## Running Locally

### Prerequisites
- Docker + docker-compose
- Go 1.22+
- AWS CLI (for SQS queue creation)

### Step 1: Start Infrastructure

```bash
sudo docker-compose up -d
```

Starts PostgreSQL (with schema) and ElasticMQ (SQS-compatible).

### Step 2: Create SQS Queues

```bash
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name jobs --region us-east-1

AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name completions --region us-east-1
```

### Step 3: Run the API

```bash
go run .
```

### Step 4: Run the Poller

```bash
go run ./poller/
```

### Step 5: Run the Executor

```bash
go run ./executor/
```

### Step 6: Schedule a Job

```bash
# Schedule a job 30 seconds from now
SCHEDULED=$(date -u -d '+30 seconds' +%Y-%m-%dT%H:%M:%SZ)

curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d "{\"command\": {\"type\": \"http\", \"url\": \"https://example.com/webhook\"}, \"scheduled_at\": \"$SCHEDULED\"}"
```

Response:
```json
{"job_id": 1, "scheduled_at": "2026-06-23T10:30:00Z", "status": "scheduled"}
```

Within 30 seconds, the executor outputs:
```
Executing job 1: map[type:http url:https://example.com/webhook]
  Command: {"type":"http","url":"https://example.com/webhook"}
Job 1 completed in 500ms
```

### Schedule a Recurring Job

```bash
curl -X POST http://localhost:8080/jobs/recurring \
  -H "Content-Type: application/json" \
  -d '{"command": {"type": "notify", "channel": "slack", "message": "Daily standup!"}, "cron_expr": "0 9 * * *"}'
```

## Project Structure

```
job-schedular/
├── main.go              # Web API (schedule jobs)
├── poller/main.go       # Polls DB, pushes to SQS, replenishes recurring
├── executor/main.go     # Pulls from SQS, executes jobs
├── models/job.go        # Job + RecurringJob + JobCompletion models
├── store/postgres.go    # DB operations (insert, pick with SKIP LOCKED, update)
├── queue/sqs.go         # SQS push/pull/delete operations
├── migration/schema.sql # Tables + partial index
├── docker-compose.yaml  # PostgreSQL + ElasticMQ (SQS)
└── README.md
```

## Cleanup

```bash
sudo docker-compose down -v
```
