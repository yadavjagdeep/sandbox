# Multi-Tier Storage

An order management system demonstrating tiered storage for cost optimization. Inspired by how Amazon handles order history — recent orders are fast to access, older orders move to cheaper storage, and very old orders are archived.

## The Problem

A growing e-commerce platform accumulates millions of orders over time. Keeping all orders in a relational database is expensive:
- SQL databases are costly (compute + storage + IOPS)
- Indexes bloat as tables grow
- Queries slow down with millions of rows
- 95% of queries are for recent orders (last 6 months)

## The Solution

Move data to progressively cheaper storage based on age. The application routes reads to the correct tier automatically using the timestamp embedded in the order ID.

```
┌─────────────────────────────────────────────────────────────────┐
│                         Read Path                                │
│                                                                 │
│  GET /orders/:id                                                │
│    → Extract timestamp from snowflake ID (one bit shift)        │
│    → age < 6 months   → PostgreSQL (hot)                       │
│    → age < 2 years    → DynamoDB (warm)                        │
│    → age >= 2 years   → "Archived, not available" (cold)       │
└─────────────────────────────────────────────────────────────────┘
```

## Storage Tiers

| Tier | Storage | Duration | Data Format | Access Speed | Cost |
|------|---------|----------|-------------|--------------|------|
| Hot | PostgreSQL | 0–6 months | Normalized (orders, items, payments tables) | ~1ms | $$$ |
| Warm | DynamoDB | 6 months–2 years | Single denormalized JSON document | ~5ms | $$ |
| Cold | S3 | 2+ years | JSON file per order | N/A (no API access) | $ |

## Design Decisions

### 1. Snowflake ID for Tier Routing

Every order gets a snowflake ID with an embedded timestamp:

```
| 41 bits: timestamp (ms) | 10 bits: machine ID | 12 bits: sequence |
```

To route a request, extract the timestamp with a single bit shift:

```go
timestamp := (id >> 22) + epoch
age := time.Since(timestamp)
// age < 6 months → SQL, age < 2 years → DynamoDB, else → archived
```

No cascading lookups. No routing table. No extra database call. The ID itself tells you where the data lives.

### 2. Why Not Redis for Hot Tier?

Storing 6 months of full order data in Redis would cost more than PostgreSQL. Redis is great for caching, not as a primary store for complex relational data with transactions.

### 3. Single JSON Document in Warm Tier

An order in SQL is spread across multiple tables (orders, order_items, payments). When it moves to warm, everything is combined into a single JSON document. One read returns everything — no joins needed for old orders.

### 4. Cold Tier: No Direct API Access

Orders older than 2 years are archive-only. Why?
- S3 reads cost money per request + egress
- Users almost never look at 2+ year old orders
- For compliance/legal needs: spin up an ephemeral database, load from S3, query, destroy after use

### 5. Data Movement: CDC + TTL (Production Design)

In production, data moves between tiers without loading the live database:

```
SQL (hot) ──CDC (tails WAL)──→ Redis (3-day TTL, bridge)
                                    │
                                    └── Nightly job: stitch → DynamoDB (warm)

SQL TTL fires (6 months + 3 days) → deletes from SQL

DynamoDB (warm) ── TTL (2 years) + DynamoDB Streams ──→ S3 (cold)
```

Key points:
- CDC tails the PostgreSQL WAL — zero load on the live database
- Redis bridge (3-day TTL) covers the transition window between SQL deletion and DynamoDB persistence
- DynamoDB's native TTL auto-deletes expired items; Streams capture the deletion to write to S3
- No batch jobs scanning large tables

### 6. Accepted Tradeoff: Brief Unavailability Window

Between TTL deleting from SQL and the nightly job persisting to DynamoDB, there's a brief window where data might be in neither tier. We accept this because:
- The Redis bridge (in production) reduces this to near-zero
- It's orders older than 6 months — not time-critical
- The window is at most 3 days (Redis TTL)

### 7. Why DynamoDB Over MongoDB?

- Native TTL support (auto-deletes expired items)
- DynamoDB Streams (CDC for warm → cold movement)
- Pay-per-request billing — zero cost when not accessed
- No server to manage

### 8. Updates: Immutable After Demotion

Once an order moves to warm or cold, it's immutable. Active orders (status changes, refunds) only happen in the hot tier. By the time an order is 6 months old, it's settled.

## Project Structure

```
multi-tier-storage/
├── main.go                 # API server entry point
├── models/order.go         # Data models (normalized + denormalized)
├── snowflake/snowflake.go  # ID generator + age extraction
├── tier/
│   ├── router.go           # Snowflake age → tier routing
│   ├── hot.go              # PostgreSQL store
│   ├── warm.go             # DynamoDB store
│   └── cold.go             # S3 store
├── handler/order.go        # HTTP handlers
├── stitcher/main.go        # Nightly job: SQL → DynamoDB
├── seeder/main.go          # Test tool: create backdated orders
├── migration/schema.sql    # PostgreSQL schema
├── docker-compose.yaml     # Local infra (Postgres + DynamoDB + MinIO)
└── README.md
```

## How to Run

### Prerequisites

- Go 1.22+
- Docker + docker-compose
- AWS CLI (for DynamoDB table creation)

### Step 1: Start Infrastructure

```bash
sudo docker-compose up -d
```

This starts:
- PostgreSQL on port 5432 (auto-runs schema.sql)
- DynamoDB Local on port 8000
- MinIO (S3-compatible) on port 9000

### Step 2: Create DynamoDB Table

```bash
AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
aws dynamodb create-table \
    --endpoint-url http://localhost:8000 \
    --table-name orders_warm \
    --attribute-definitions AttributeName=id,AttributeType=N \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region us-east-1
```

### Step 3: Run the Server

```bash
go run .
```

Output:
```
Multi-tier storage server on :8080
⇨ http server started on [::]:8080
```

## Testing Each Tier

### Test 1: Hot Tier (Create + Read a Recent Order)

```bash
# Create an order
curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1001,
    "items": [{"product_name": "Laptop", "quantity": 1, "price": 999.99}],
    "payment": {"method": "credit_card"}
  }' | python3 -m json.tool
```

Response:
```json
{
    "order_id": "343051188830212096",
    "status": "created",
    "tier": "hot (SQL)",
    "total": 999.99
}
```

```bash
# Fetch it — should come from SQL (hot)
curl -s http://localhost:8080/orders/343051188830212096 | python3 -m json.tool
```

Response:
```json
{
    "tier": "hot (SQL)",
    "order": {
        "id": 343051188830212096,
        "user_id": 1001,
        "status": "created",
        "total_amount": 999.99,
        "items": [...],
        "payments": [...]
    }
}
```

### Test 2: Warm Tier (Demote Old Orders to DynamoDB)

```bash
# Seed orders that are 7 months old (in SQL but should be in warm tier)
go run ./seeder/ --age 7m --count 2
```

Output:
```
Created order: 266949849063100416 (age: 7m, tier: warm (DynamoDB))
Created order: 266949853257404416 (age: 7m, tier: warm (DynamoDB))
```

```bash
# Try to fetch — router sends to DynamoDB, but data isn't there yet
curl -s http://localhost:8080/orders/266949849063100416 | python3 -m json.tool
```

Response:
```json
{
    "error": "order not found in warm tier"
}
```

```bash
# Run the stitcher to move old orders from SQL → DynamoDB
go run ./stitcher/
```

Output:
```
Stitcher complete: demoted 2 orders from hot → warm
```

```bash
# Fetch again — now found in DynamoDB
curl -s http://localhost:8080/orders/266949849063100416 | python3 -m json.tool
```

Response:
```json
{
    "tier": "warm (DynamoDB)",
    "order": {
        "id": 266949849063100416,
        "user_id": 1001,
        "status": "completed",
        "total_amount": 99.99,
        "items": [...],
        "payments": [...],
        "ttl": 1844861823
    }
}
```

### Test 3: Cold Tier (Archived Orders Rejected)

```bash
# Seed a 3-year-old order
go run ./seeder/ --age 3y --count 1
```

```bash
# Fetch — router rejects with "archived"
curl -s http://localhost:8080/orders/<order_id> | python3 -m json.tool
```

Response:
```json
{
    "error": "order archived, not available for direct access",
    "tier": "cold (S3)"
}
```

## Cost Comparison

Assuming 10M orders total, 50K new orders/month:

| Approach | Monthly Cost (estimated) |
|----------|------------------------|
| All in PostgreSQL (RDS) | ~$500-1000/month (large instance for 10M rows) |
| Tiered (this approach) | ~$50-100/month (small RDS + DynamoDB on-demand + S3) |

The savings come from:
- SQL only holds 300K orders (6 months) instead of 10M
- Smaller RDS instance needed
- DynamoDB pay-per-request — old orders rarely accessed
- S3 archive is pennies per GB

## What's Not Implemented (Production Considerations)

| Feature | Status | Production Approach |
|---------|--------|-------------------|
| CDC (SQL → S3 staging) | Not implemented | Debezium tailing PostgreSQL WAL |
| Redis bridge | Not implemented | 3-day TTL Redis, CDC writes here |
| SQL TTL deletion | Not implemented | Cron: `DELETE WHERE created_at < 6 months` |
| DynamoDB TTL | TTL field set | Enable via `update-time-to-live` on table |
| DynamoDB Streams → S3 | Not implemented | Lambda consuming stream, writes to S3 |
| Warm → Cold movement | Not implemented | DynamoDB TTL + Streams consumer |

## Cleanup

```bash
sudo docker-compose down -v
```
