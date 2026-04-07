# pgstore

Go API + PostgreSQL project for learning database internals hands-on.

## Quick Start

```bash
./start.sh
```

Starts all Postgres containers (primary, replica, shard1, shard2) and the Go API on :8080.

## Architecture

```
curl → Go API (:8080) → Primary (:5432)  — writes
                       → Replica (:5433)  — reads (auto-replicated from primary)
                       → Shard1  (:5434)  — even ID products
                       → Shard2  (:5435)  — odd ID products
```

## API Endpoints

```
POST /products              — create product (primary)
GET  /products              — list products (replica)
POST /orders                — create order (primary)
GET  /orders                — list orders (replica)

POST /sharded/products      — create product (routed to shard by ID)
GET  /sharded/products      — list all products (queries both shards, merges)
GET  /sharded/products/:id  — get product by ID (routed to correct shard)
```

## What Was Learned

### Indexes & Query Plans
- EXPLAIN ANALYZE to see how Postgres executes queries
- Seq Scan (full table read) vs Index Scan (direct lookup) vs Bitmap Scan (batch page reads)
- Composite indexes: column order matters, like a phone book (last name, first name)
- Index tradeoff: faster reads, slower writes, more disk space

### MVCC (Multi-Version Concurrency Control)
- UPDATE creates a new row version, old one stays until vacuum
- Each row has xmin (created by txn) and xmax (replaced by txn)
- Readers never block writers, writers never block readers
- Write locks only exist to prevent lost updates on relative changes (price = price + 1)
- Blocked session re-reads latest committed value when unblocked

### Postgres vs MySQL
- Postgres index stores ctid (page, offset) — one hop to data
- MySQL secondary index stores primary key value — two hops (double lookup)
- Postgres: new row version in main table, vacuum cleans up
- MySQL: update in place, old version in undo log

### Partitioning (Phase 2)
- Splits data within a single database by range, list, or hash
- Orders partitioned by ordered_at (quarterly)
- Partition pruning: Postgres skips partitions that can't match the query
- Full scan 38ms/200k rows vs single partition 6ms/50k rows
- DROP partition is instant vs DELETE row-by-row
- Parent table is a shell with no data, children hold actual rows

### Replication (Phase 3)
- Primary handles writes, replica streams WAL and handles reads
- Streaming replication: replica continuously receives WAL from primary
- Replica is read-only — rejects INSERT/UPDATE/DELETE
- If primary dies: reads still work, writes fail until failover
- pg_stat_replication shows lag and sync status

### Sharding (Phase 4)
- Splits data across multiple independent databases
- Routing in application: GetShard(id) using id % 2
- Odd/even sequences prevent ID conflicts across shards
- Single-key lookup hits one shard (fast)
- Get-all queries all shards and merges (slower)
- Cross-shard JOINs, sorting, pagination, aggregation all handled in app code
- Shard key selection is critical — most queries should hit single shard

## Useful psql Commands

```sql
\dt                              -- list tables
\di                              -- list indexes
\d tablename                     -- describe table
EXPLAIN ANALYZE SELECT ...       -- show query execution plan
SELECT xmin, xmax, * FROM ...   -- see MVCC version info
SELECT * FROM pg_stat_replication -- check replication status
SELECT relname, n_dead_tup FROM pg_stat_user_tables -- dead rows
VACUUM tablename                 -- clean up dead rows
ANALYZE tablename                -- update query planner stats
```

## Docker Commands

```bash
sudo docker-compose up -d        -- start all containers
sudo docker-compose down         -- stop (keep data)
sudo docker-compose down -v      -- stop and DELETE all data
sudo docker start pgstore-primary pgstore-replica pgstore-shard1 pgstore-shard2
sudo docker exec -it pgstore-primary psql -U pgstore  -- connect to primary
sudo docker exec -it pgstore-shard1 psql -U pgstore   -- connect to shard1
```
