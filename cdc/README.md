# CDC (Change Data Capture) — Deep Dive

## What is CDC?

Capturing row-level changes (INSERT, UPDATE, DELETE) from a database's internal log and streaming them as events to downstream consumers — without polling.

Instead of asking "what changed?", you listen to the database's own change log (WAL).

---

## Why CDC over alternatives?

| Approach | Problem |
|----------|---------|
| Polling (`WHERE updated_at > X`) | Lag, misses deletes, adds load, breaks if timestamp column forgotten |
| Dual writes (app writes DB + publishes event) | Inconsistency if one fails |
| Triggers (write to outbox on every mutation) | Write amplification, couples logic to DB |
| **CDC (read from WAL)** | Guaranteed, ordered, complete stream. Zero load on source. |

---

## WAL (Write-Ahead Log) — The Foundation

Every write in Postgres follows this path:

```
Client: UPDATE users SET email = 'new@x.com' WHERE id = 7;

1. WAL record → WAL buffer (RAM, briefly)
2. WAL buffer → fsync → WAL segment file (DISK) ← DURABILITY POINT
3. Transaction ACK to client
4. Data page modified in buffer pool (RAM, "dirty page")
5. Checkpoint: dirty page flushed to data file (DISK, async, later)
```

Key insight: WAL hits disk BEFORE the client gets an ACK. Data files are lazily maintained.
If crash happens after step 2 but before step 5: Postgres replays WAL on restart. Nothing lost.

### Why WAL and not direct write?

- WAL = sequential I/O (fast, always appending)
- Data files = random I/O (slow, different pages scattered on disk)
- Critical path (commit) uses fast sequential writes; random writes are batched via checkpoints

---

## Physical vs Logical WAL

```
Physical WAL:
  "page 42, offset 128, write these 64 bytes"
  → Used for: crash recovery, physical replicas (byte-for-byte copy)
  → Useless for CDC (can't extract row-level meaning)

Logical Decoding:
  "table: users, op: UPDATE, old: {email: 'old'}, new: {email: 'new'}"
  → Used for: CDC, logical replication
  → Requires: wal_level=logical (records extra tuple data in WAL)
```

Logical decoding reads the SAME WAL segments and interprets them at a higher level.

---

## Key Postgres Concepts for CDC

### Replication Slot

A bookmark in the WAL. Postgres keeps WAL segments around until the slot's consumer ACKs them.

- Guarantees no missed events (even if consumer is offline temporarily)
- Risk: abandoned slot → WAL accumulates → disk fills up

### Publication

A filter that defines which tables' changes are exposed to logical replication consumers.

```sql
CREATE PUBLICATION cdc_pub FOR TABLE users;
-- Only users table changes are streamed

CREATE PUBLICATION cdc_pub FOR ALL TABLES;
-- Everything (dangerous in prod)
```

### Replica Identity

Controls what Postgres includes in UPDATE/DELETE WAL messages:

```sql
-- Default: only PK sent on UPDATE old-image and DELETE
-- Result: DELETE shows id=1, everything else NULL

ALTER TABLE users REPLICA IDENTITY FULL;
-- Full row sent on UPDATE (before + after) and DELETE
-- Result: DELETE shows all columns, UPDATE shows old AND new
```

### LSN (Log Sequence Number)

Position in the WAL — like a Kafka offset. Format: `0/16A2B30`.

Your consumer tracks this and reports back to Postgres: "I've processed up to here, you can reclaim older WAL."

---

## Replication Protocol

CDC consumers connect using the **streaming replication protocol** (not regular SQL).
From Postgres's perspective, your CDC app IS a replica — same protocol, same heartbeats.

```
Regular SQL connection:   request → response, request → response
Replication connection:   START_REPLICATION → continuous stream of WAL data
```

That's why `database/sql` and `lib/pq` can't do CDC — they only speak SQL protocol.
You need `pgconn` (from pgx) which supports replication mode.

Connection string: `postgres://user:pass@host:port/db?replication=database`

---

## pgoutput Protocol Messages

When streaming, Postgres sends these message types:

| Message | When | Contains |
|---------|------|----------|
| Relation | First time a table appears (or schema changes) | Table name, column names, column types |
| Begin | Start of a transaction | Transaction ID (xid) |
| Insert | Row inserted | Full new tuple |
| Update | Row updated | Old tuple (if REPLICA IDENTITY FULL) + new tuple |
| Delete | Row deleted | Old tuple (key only, or full if REPLICA IDENTITY FULL) |
| Commit | Transaction committed | Commit LSN, timestamp |

Important: Relation messages must be cached — Insert/Update/Delete reference columns by position, you need the Relation metadata to know which column is which.

---

## Heartbeats (Standby Status Updates)

Your consumer must periodically send: "I'm alive, I've processed up to LSN X."

- If you don't send within `wal_sender_timeout` (default 60s) → Postgres drops connection
- The LSN you report tells Postgres it can reclaim old WAL segments
- Postgres also sends PrimaryKeepaliveMessages — if `ReplyRequested` is set, respond immediately

---

## Architecture

```
┌──────────────┐    replication protocol     ┌──────────────────┐
│  PostgreSQL  │◀────────────────────────────│  Go CDC Consumer │
│              │    (streaming WAL)           │  (pglogrepl)     │
│ wal_level=   │                             │                  │
│   logical    │    heartbeats (LSN ACK)     │  decodes events  │
│              │◀────────────────────────────│  prints/forwards │
└──────────────┘                             └──────────────────┘
```

---

## Stateless vs Stateful — Why DB Scaling is Hard

API servers: stateless → share nothing → scale by adding pods behind LB.
DB servers: stateful → own data → need coordination to scale.

You can't round-robin writes to multiple DB instances without consensus.

### Multi-node DB coordination approaches:

| Approach | Who writes | How they sync | Example |
|----------|-----------|---------------|---------|
| Single-leader | One node | WAL streaming to replicas | Postgres, MySQL |
| Consensus (Raft) | Any node (with majority agreement) | Raft log replication | CockroachDB, etcd |
| Shared storage | One primary writes | Quorum writes to shared storage | Aurora |

---

## CAP Theorem

During a network partition, choose:
- **CP** (Consistency): refuse requests rather than give wrong data (Postgres sync repl, CockroachDB)
- **AP** (Availability): always respond, might be stale (DynamoDB default, Cassandra)

When network is fine: you get both C and A. CAP only forces a choice during partitions.

Real systems make different choices per operation:
- Writes → usually CP (can't lose data)
- Primary reads → CP (need latest)
- Replica/cache reads → AP (staleness acceptable)

### Decision framework:

```
"What's worse: wrong answer or no answer?"

Wrong answer worse → CP (bank balance, distributed lock)
No answer worse   → AP (search results, product catalog)
```

---

## REPLICA IDENTITY explained

```sql
-- Default (only PK in old image):
DELETE → {id: 1, name: NULL, email: NULL}
UPDATE → old: {id: 1}, new: {id: 1, name: alice, email: new@x.com}

-- FULL (complete old row):
DELETE → {id: 1, name: alice, email: alice@x.com}
UPDATE → old: {id: 1, name: alice, email: old@x.com}, new: {id: 1, name: alice, email: new@x.com}
```

Tradeoff: FULL makes WAL records larger. Only enable on tables where you need before-image.

---

## Running This Project

```bash
# Start Postgres with logical replication enabled
sudo docker-compose up

# Run the CDC consumer
go run main.go

# In another terminal, connect and make changes
sudo docker exec -it cdc_postgres_1 psql -U cdc -d cdcdb

INSERT INTO users (name, email) VALUES ('alice', 'alice@example.com');
UPDATE users SET email = 'alice-new@example.com' WHERE name = 'alice';
DELETE FROM users WHERE name = 'alice';
```

---

## Production: Debezium vs Custom

| | Custom (pglogrepl) | Debezium |
|---|---|---|
| Language | Go | Java (runs as Docker) |
| Output | Whatever you want | Kafka topics |
| Handles reconnection | You build it | Built-in |
| Schema evolution | You build it | Built-in + Schema Registry |
| Snapshotting | You build it | Built-in |
| Use case | Low-latency, specific table, minimal deps | General CDC pipeline at scale |

Your Go service consuming from Debezium just reads Kafka — never touches the replication protocol.

---

## Key Configs (docker-compose)

```yaml
command:
  - "postgres"
  - "-c"
  - "wal_level=logical"          # enable logical decoding (extra tuple data in WAL)
  - "-c"
  - "max_replication_slots=4"    # max concurrent CDC consumers / replicas
  - "-c"
  - "max_wal_senders=4"          # max concurrent replication connections
```

---

## Gotchas

1. **Replication slot already exists** — if you restart your app, drop the slot first or handle the error
2. **Abandoned slots fill disk** — Postgres holds WAL forever if slot isn't consumed
3. **Relation messages** — cache them; Insert/Update/Delete reference columns by position
4. **Column values are text-encoded bytes** — decode using relation metadata
5. **wal_level change requires restart** — can't change at runtime
6. **REPLICA IDENTITY FULL** — needed for old images, increases WAL size
