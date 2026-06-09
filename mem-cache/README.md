# mem-cache

A distributed in-memory cache built from scratch in Go. Implements consistent hashing, LRU eviction, TTL expiration, and a custom TCP protocol — inspired by Redis internals.

## Architecture

```
┌──────────┐       ┌──────────┐       ┌──────────┐
│  Node 1  │◄─────►│  Node 2  │◄─────►│  Node 3  │
│  :7071   │       │  :7072   │       │  :7073   │
│          │       │          │       │          │
│ ┌──────┐ │       │ ┌──────┐ │       │ ┌──────┐ │
│ │Cache │ │       │ │Cache │ │       │ │Cache │ │
│ │(map) │ │       │ │(map) │ │       │ │(map) │ │
│ └──────┘ │       │ └──────┘ │       │ └──────┘ │
└─────▲────┘       └─────▲────┘       └─────▲────┘
      │                   │                   │
      └───────────────────┼───────────────────┘
                          │
                    ┌─────┴─────┐
                    │  Client   │
                    │(any node) │
                    └───────────┘
```

Each node is identical — same binary, same code. Clients connect to any node; the node routes the request to the correct owner via consistent hashing.

## Sequence Diagrams

### PUT Request (local)

```
Client              Node 1 (owner)
  │                      │
  │── PUT key 60 5 ────►│
  │   world              │
  │                      │── hash("key") → ring → "Node 1" (me)
  │                      │── check memory budget
  │                      │── evict if needed
  │                      │── store in map
  │◄──── +OK ───────────│
  │                      │
```

### GET Request (forwarded)

```
Client              Node 2                    Node 3 (owner)
  │                   │                           │
  │── GET key ──────►│                           │
  │                   │── ring.GetNode("key")     │
  │                   │   → "Node 3" (not me)     │
  │                   │                           │
  │                   │── TCP connect ───────────►│
  │                   │── GET key ──────────────►│
  │                   │                           │── lookup in map
  │                   │◄── $5\r\nworld ──────────│
  │                   │                           │
  │◄── $5\r\nworld ──│                           │
  │                   │                           │
```

### Node Join

```
Node 2 (new)              Node 1 (seed)           Node 3 (existing)
  │                           │                       │
  │── JOIN localhost:7072 ──►│                       │
  │                           │── add N2 to ring     │
  │                           │── JOINED N2 ────────►│
  │                           │                       │── add N2 to ring
  │◄── NODES N1,N2,N3 ──────│                       │
  │── add all to ring        │                       │
  │                           │                       │
```

### Heartbeat & Failure Detection

```
Node 1                    Node 2                    Node 3
  │                         │                         │
  │── PING ────────────────►│                         │
  │◄── PONG ────────────────│                         │
  │                         │                         │
  │── PING ───────────────────────────────────────────►│ (no response)
  │── PING ───────────────────────────────────────────►│ (no response)
  │── PING ───────────────────────────────────────────►│ (no response)
  │                         │                         │
  │ (3 misses → N3 dead)   │                         │
  │── remove N3 from ring  │                         │
  │── LEFT N3 ────────────►│                         │
  │                         │── remove N3 from ring  │
  │                         │                         │
```

## Design Decisions

### 1. Storage

| Decision | Rationale |
|----------|-----------|
| `map[string]Entry` | Go's built-in hash table, O(1) operations |
| Single data goroutine | No locks needed — Redis model. CPU is never the bottleneck, network is |
| Entry = `{value, expireAt, accessedAt}` | All metadata per-key for TTL + LRU |
| Byte budget tracking (`usedBytes`) | Accurate memory accounting for eviction trigger |

**How Redis does it:** Custom `dict` with incremental rehashing, separate `expires` dict for TTL. Single-threaded event loop for same reason — eliminates lock contention.

### 2. Eviction

| Decision | Rationale |
|----------|-----------|
| Approximated LRU (sample 5) | Near-zero overhead, no linked list, protects hot keys |
| Trigger: `usedBytes + newEntry > maxBytes` | Evict before inserting, not after |
| Loop eviction until room available | One large value may need multiple small evictions |

**How Redis does it:** Same algorithm (`maxmemory-samples` = 5 by default). Also offers LFU, random, volatile-ttl as configurable policies.

**Alternative policies considered:**

| Policy | Pros | Cons |
|--------|------|------|
| True LRU (linked list) | Perfect accuracy | 16 bytes/key overhead, lock contention |
| LFU | Better for skewed workloads | More complex (logarithmic counter + decay) |
| Random | Simplest, zero per-access overhead | Can evict hot keys |
| **Approx LRU (chosen)** | Near-zero overhead, protects hot keys | ~5% less accurate than true LRU |

### 3. TTL

| Decision | Rationale |
|----------|-----------|
| Lazy expiration on GET | No expired key ever returned to client |
| Active sweep every 100ms | Prevents memory leaks from cold expired keys |
| Adaptive sweep (repeat if >25% expired) | Self-balancing — more CPU only when more garbage |
| Absolute timestamp (`expireAt`) | No recomputation needed, just compare with `now` |

**How Redis does it:** Identical approach — `expireIfNeeded()` on access + `activeExpireCycle` every 100ms with 25% threshold.

### 4. Communication

| Decision | Rationale |
|----------|-----------|
| Custom text-based TCP protocol | Minimal overhead, debuggable with `nc`/`telnet` |
| Length-prefixed values | Binary-safe (value can contain `\r\n`) |
| Goroutine per connection | Go's strength — handles 10K+ concurrent connections |
| Single port for client + cluster traffic | Simpler deployment, same parser handles both |

**Protocol format:**

```
Commands:
  PUT <key> <ttl> <valueLen>\r\n<value>\r\n
  GET <key>\r\n
  DEL <key>\r\n

Responses:
  +OK\r\n              (success)
  $<len>\r\n<data>\r\n (GET hit)
  $-1\r\n              (GET miss)
  -ERR <msg>\r\n       (error)

Cluster:
  JOIN <nodeID>\r\n
  NODES <n1>,<n2>,<n3>\r\n
  JOINED <nodeID>\r\n
  LEFT <nodeID>\r\n
  PING\r\n / PONG\r\n
```

**Why not HTTP:** ~200-500 bytes header overhead per request vs ~20 bytes for custom protocol. For a cache where every microsecond matters, this is significant.

**Why not gRPC:** Extra dependency (protobuf), serialization overhead. Raw TCP is simpler and faster for this use case.

### 5. Concurrency

| Decision | Rationale |
|----------|-----------|
| Single data goroutine (channel-based) | No locks, no races, no deadlocks. Redis model. |
| N connection goroutines (I/O only) | Parse bytes, send commands, write responses |
| Command channel (conn → data goroutine) | Serializes all data access |
| Per-command response channel | Data goroutine sends result back to correct conn |

**How Redis does it:** Single-threaded event loop (epoll). Same principle — all data access is serialized. Redis 6+ added I/O threads for socket read/write, but data operations remain single-threaded.

**Why single-threaded works:** In-memory hash table operation takes ~100ns. Network round trip takes ~100,000ns. CPU is idle 99.9% of the time waiting for network. One goroutine can handle more ops/sec than the network can deliver.

### 6. Consistent Hashing

Consistent hashing is a standalone topic applicable far beyond caching — load balancers, databases, CDNs, message brokers, and distributed file systems all use it.

#### The Problem

Naive approach: `node = hash(key) % numNodes`

```
3 nodes: hash("user:1") % 3 = 1 → node 1
4 nodes: hash("user:1") % 4 = 0 → node 0  ← MOVED!
```

Adding one node changes where almost every key maps. With K keys, ~K keys get remapped. This invalidates the entire cache — a catastrophic "thundering herd" hits the database.

#### The Solution: Hash Ring

Imagine a circle numbered 0 to 2^32 (4.29 billion positions):

```
           0
         ╱   ╲
       ╱       ╲
     ╱    Ring    ╲
    │               │
    │ A (pos 1000)  │
    │         B (pos 5000)
     ╲             ╱
       ╲         ╱
         ╲     ╱  C (pos 8000)
           ╲ ╱
```

**Rules:**
1. Hash each node's ID → get a position on the ring
2. Hash each key → get a position on the ring
3. Walk clockwise from the key's position → first node you hit owns that key

**Key insight:** No modulo anywhere. The hash function output IS the position. Since CRC32 outputs uint32, every possible hash is already a valid position on the ring (0 to 2^32).

#### What Happens When a Node is Added

```
Before: Nodes at positions [1000(A), 5000(B), 8000(C)]
        Key at position 4500 → walks clockwise → hits B(5000) → owned by B

Add node D at position 3000:
        Nodes at [1000(A), 3000(D), 5000(B), 8000(C)]
        Key at position 4500 → walks clockwise → hits B(5000) → STILL owned by B ✓
        Key at position 2500 → was owned by B → now owned by D (moved)
```

Only keys between A(1000) and D(3000) move to D. Everything else stays. That's ~K/N keys moved vs ~K keys with modulo.

#### What Happens When a Node is Removed

```
Before: Nodes at [1000(A), 3000(D), 5000(B), 8000(C)]
Remove D:
        Nodes at [1000(A), 5000(B), 8000(C)]
        Keys that were on D → now owned by B (next clockwise)
```

Only D's keys move. All other keys stay on their current nodes.

#### Virtual Nodes

**Problem:** With 3 physical nodes and 3 positions on the ring, the arcs between them can be wildly uneven:

```
Bad distribution (3 positions):
A──────────────────B───C────────────────────A (wrap)
     60% of keys    10%      30% of keys
Node A handles 60% of all keys — hot node!
```

**Solution:** Give each physical node many positions (virtual nodes):

```
ring.AddNode("nodeA") generates:
  hash("nodeA-0")   = position 120
  hash("nodeA-1")   = position 3500
  hash("nodeA-2")   = position 7200
  ... (128 total, scattered across the ring)
```

With 3 nodes × 128 virtual nodes = 384 positions evenly scattered. Each node owns ~33% of the key space.

**Why 128?** Diminishing returns — 128 gives nearly perfect distribution. 256 is marginally better but doubles memory for the ring. 5-10 is too few for even distribution.

**Virtual nodes also help during topology changes:** When a new node joins, its 128 positions are scattered across the ring, taking small chunks from ALL existing nodes evenly — no single node loses a disproportionate share.

#### The Lookup Algorithm

```go
func (r *Ring) GetNode(key string) string {
    hash := crc32(key)                    // O(1) — key → position
    idx := binarySearch(ring, hash)       // O(log N) — find next position clockwise
    return nodeMap[ring[idx]]             // O(1) — position → physical node
}
```

Total: O(log N) where N = total virtual nodes on ring. For 384 positions, that's ~9 comparisons. Effectively instant.

#### Data Structure

```go
type Ring struct {
    hashRing []uint32            // sorted positions on the ring [120, 890, 1450, ...]
    nodeMap  map[uint32]string   // position → physical node {"120": "nodeA:7071"}
    replicas int                 // virtual nodes per physical node (128)
}
```

- `hashRing` is always kept sorted after AddNode — enables binary search
- `nodeMap` translates positions back to physical node addresses
- Wrap-around: if key's hash > all positions, wrap to index 0 (the ring is circular)

#### Hash Function Choice

| Function | Speed | Distribution | Notes |
|----------|-------|--------------|-------|
| CRC32 (chosen) | Very fast | Good | Go stdlib, no dependencies |
| FNV-1a | Fast | Good | Alternative stdlib option |
| MurmurHash3 | Fast | Excellent | External dependency |
| MD5/SHA | Slow | Perfect | Overkill for this use case |

CRC32 is the pragmatic choice — fast, good enough distribution, zero dependencies.

#### Consistent Hashing Applications Beyond Caching

| System | Use Case |
|--------|----------|
| Load Balancers | Route requests to servers (sticky sessions) |
| DynamoDB | Partition data across storage nodes |
| Cassandra | Determine which node stores a partition |
| Kafka | Partition assignment to consumers |
| CDNs (Akamai) | Route content to edge servers |
| Discord | Assign guilds to server processes |

#### Complexity Summary

| Operation | Time | Description |
|-----------|------|-------------|
| GetNode | O(log N) | Binary search on sorted ring |
| AddNode | O(R log N) | Insert R virtual nodes, re-sort |
| RemoveNode | O(N) | Filter out R positions, rebuild |
| Memory | O(N × R) | N nodes × R replicas positions + map entries |

Where N = physical nodes, R = replicas (128).

### 7. Cluster Membership

| Decision | Rationale |
|----------|-----------|
| Seed + Join protocol | Dynamic, simple (~80 lines), no external dependencies |
| Heartbeat every 2s, 3 misses = dead | Quick failure detection without excessive traffic |
| Broadcast on join/leave | All nodes stay in sync |

**Alternative approaches considered:**

| Approach | Pros | Cons | Used by |
|----------|------|------|---------|
| Static config | Simplest, no race conditions | Restart all nodes to change topology | Memcached clients |
| **Seed + Join (chosen)** | Dynamic, moderate complexity | Seed needed for bootstrap | Elasticsearch |
| Gossip (SWIM) | Fully decentralized, scales to 1000s | Complex (~250 lines), eventual consistency | Cassandra, Consul |

### 8. Request Routing

**Three possible approaches:**

| Approach | How it works | Latency | Client complexity |
|----------|-------------|---------|-------------------|
| Client-side routing | Client knows ring, routes directly | 1 hop | High (needs ring state) |
| Central proxy | LB in front routes to correct node | 2 hops | Low (dumb client) |
| **Node forwarding (chosen)** | Any node routes internally | 1-2 hops | Low (dumb client) |

**Chosen:** Node forwarding — client connects to any node, node forwards if needed. Simple client (even `nc` works), no single point of failure, no client-side ring management.

**Trade-off:** Extra hop when hitting the wrong node. Optimization: connection pooling between nodes (not implemented yet).

## Project Structure

```
mem-cache/
├── main.go         # startup, config, --port/--max-mb/--join flags
├── types.go        # Entry, Command, Result structs
├── cache.go        # data goroutine, map, eviction, TTL sweep
├── protocol.go     # parser (bytes→Command) + serializer (Result→bytes)
├── server.go       # TCP listener, conn handling, routing, forwarding
├── ring.go         # consistent hashing (hash ring, virtual nodes)
├── cluster.go      # seed+join, heartbeat, failure detection
└── go.mod
```

## Usage

```bash
# Build
go build -o mem-cache .

# Start seed node
./mem-cache --port 7071 --max-mb 128

# Join more nodes
./mem-cache --port 7072 --max-mb 128 --join localhost:7071
./mem-cache --port 7073 --max-mb 128 --join localhost:7071

# Client operations (connect to any node)
echo -e "PUT user:1001 60 5\r\nhello\r" | nc localhost 7071
echo -e "GET user:1001\r" | nc localhost 7072
echo -e "DEL user:1001\r" | nc localhost 7073
```

## What I Learned

- Why Redis is single-threaded (network-bound, not CPU-bound)
- Consistent hashing vs modulo hashing (minimal key movement)
- Virtual nodes for even distribution
- Approximated LRU (random sampling beats linked lists)
- Lazy + active TTL expiration (Redis's exact algorithm)
- Custom wire protocols over TCP
- Channel-based concurrency in Go (no locks needed)
- Cluster membership without a central coordinator

## Potential Improvements

- [ ] Connection pooling between nodes (avoid TCP handshake per forward)
- [ ] Replication (leader-follower for fault tolerance)
- [ ] Key migration on node join (transfer owned keys from existing nodes)
- [ ] Smart client library (client-side ring, direct routing)
- [ ] Binary protocol option (higher throughput)
- [ ] Persistence (WAL / snapshots)
- [ ] Metrics / stats endpoint (hit rate, memory usage, key count)
