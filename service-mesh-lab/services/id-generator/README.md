# id-generator

Distributed ID generation strategies implemented in Go.

## Run

```bash
go run main.go
```

## Endpoints

```
GET /snowflake          — generate one Snowflake ID (Twitter-style)
GET /snowflake/batch    — generate 10 IDs in one call (shows sequence incrementing)
GET /central/batch      — get an ID range from central service (e.g., 1-1000)
GET /central/next       — get next single ID (uses batching internally)
```

## Why Not UUID?

UUIDs (v4) are 128-bit random values. Globally unique without coordination. Sounds perfect, but they have real problems with SQL databases:

- B-tree indexes expect sorted inserts. UUIDs are random, so every INSERT lands on a random page in the index. This causes page splits, fragmentation, and slower writes. Sequential IDs append to the end of the B-tree — fast and clean.
- UUIDs are 36 characters (or 16 bytes binary). An int64 is 8 bytes. Indexes on UUIDs are ~2x larger, which means more memory, more disk I/O, slower queries.
- JOINs on UUID foreign keys are slower than on integers.
- UUIDs are not sortable by time. You can't look at two UUIDs and know which was created first. Sequential/Snowflake IDs are time-ordered by design.

When UUIDs make sense:
- NoSQL databases (DynamoDB, Cassandra, MongoDB) — these don't use B-trees the same way. They hash the partition key anyway, so random UUIDs distribute evenly across partitions. Actually beneficial for avoiding hot partitions.
- When you need IDs generated client-side without any server coordination.
- When exposing IDs publicly and you don't want them to be guessable/enumerable.

Why the B-tree problem doesn't apply to NoSQL:
- DynamoDB hashes the partition key to decide which partition stores the data. It doesn't maintain a sorted index on the key. Random UUIDs actually help here — they spread writes evenly across partitions. Sequential IDs would cause a hot partition because all new writes go to the same partition (the one handling the latest range).
- Cassandra uses consistent hashing on the partition key. The actual key value doesn't matter for storage order — it gets hashed to a token range. Random UUIDs distribute tokens evenly across the ring.
- MongoDB with hashed shard keys behaves similarly — the key is hashed before routing to a shard.
- In all these systems, there's no B-tree being fragmented by random inserts. The storage engine is designed for distributed writes, not sorted sequential access. So the main argument against UUIDs (B-tree fragmentation) simply doesn't exist here.

## Approaches

### 1. Auto-Increment (Postgres SERIAL)

Simplest. Sequential, compact, fast for B-trees. But:
- Doesn't work across multiple databases (two nodes generate the same ID)
- Single point of failure
- Exposes ordering and volume (competitor can estimate your growth)

### 2. Central ID Service with Batching (Amazon-style)

A central service owns a counter and hands out ranges:
```
Server A requests batch → gets range 1-1000
Server B requests batch → gets range 1001-2000
Each server generates IDs locally from its range
```

Pros:
- Sequential IDs (good for B-trees)
- Few network calls (one per batch, not per ID)
- Simple to understand

Cons:
- Central service is a single point of failure
- If central service is down, no new batches — ID generation stops
- Counter must be persisted (database/disk) — if lost, IDs restart and collide
- If a server crashes, unused IDs in its batch are lost (gaps)
- Batch size tuning: too small = frequent calls, too large = bigger gaps on crash

### 3. Snowflake (Twitter-style) ← recommended for most cases

64-bit ID composed of:
```
| 41 bits: timestamp (ms) | 10 bits: machine ID | 12 bits: sequence |
```

- 41 bits timestamp: ~69 years from custom epoch
- 10 bits machine ID: supports 1024 machines
- 12 bits sequence: 4096 IDs per millisecond per machine

Pros:
- No central coordination — each machine generates independently
- Time-sortable (most significant bits are timestamp)
- Compact (int64, 8 bytes)
- High throughput (4096 IDs/ms/machine = ~4M IDs/sec/machine)
- No single point of failure
- Roughly sequential — great for B-tree indexes

Cons:
- Clock dependency — if system clock goes backward, IDs could collide. Mitigated by rejecting backward clocks.
- Machine ID assignment needs coordination (but only once at startup, not per ID)
- IDs are not strictly sequential (different machines produce interleaved IDs)

### 4. Instagram's Approach

Similar to Snowflake but uses Postgres sequences:
```
| 41 bits: timestamp | 13 bits: logical shard ID | 10 bits: sequence |
```

Sequence comes from Postgres `nextval()` instead of in-memory counter. Durable by design — no lost IDs on app restart. Tied to Postgres, which is both a strength (persistence) and limitation (dependency).

### 5. ULID

128-bit: 48-bit timestamp + 80-bit random. Lexicographically sortable, URL-safe encoding. Like UUID but sortable. Good middle ground when you need UUID-like properties but also want time ordering.

## Comparison

```
Approach          | Coordination | Sortable | Size   | Throughput | Failure Mode
------------------|-------------|----------|--------|------------|------------------
UUID v4           | None        | No       | 16B    | Unlimited  | None
Auto-increment    | Per-insert  | Yes      | 8B     | DB-limited | DB is SPOF
Central batching  | Per-batch   | Yes      | 8B     | High       | Central svc SPOF
Snowflake         | None*       | Yes      | 8B     | Very high  | Clock skew
Instagram         | Per-insert  | Yes      | 8B     | DB-limited | DB is SPOF
ULID              | None        | Yes      | 16B    | Unlimited  | None
```

*Snowflake needs machine ID assigned once, not per ID.

## When to Use What

- Single Postgres, no sharding → just use SERIAL
- Multiple databases, need sequential-ish IDs → Snowflake
- Client-side generation, no server calls → UUID or ULID
- NoSQL with hash partitioning → UUID (random distribution is actually good)
- Need time ordering + uniqueness + compactness → Snowflake
- Public-facing IDs that shouldn't be guessable → UUID or ULID

## Implementation Notes

- Snowflake's sequence resets each millisecond. If 4096 IDs are exhausted in one ms, it waits for the next ms.
- The custom epoch (instead of Unix epoch) keeps IDs smaller by starting the timestamp from a recent date.
- Mutex protects the generator — only one goroutine generates at a time. Under extreme load, this could be a bottleneck. Solution: per-goroutine generators with different machine IDs.
- Central batching counter must be persisted (Postgres, Redis with AOF, or disk). In-memory counter loses state on restart.
