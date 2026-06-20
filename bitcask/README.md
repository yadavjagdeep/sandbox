# Bitcask — A Log-Structured Key-Value Store

An implementation of the Bitcask storage engine — a log-structured hash table designed for high write throughput on spinning disks (HDDs). Originally created as the backend for the Riak distributed database.

## What Problem Does It Solve?

Traditional databases (B-tree based) perform **random writes** — updating data pages, indexes, and WAL entries scattered across disk. On HDDs, each random write requires a physical disk seek (~10ms). This makes writes slow.

Bitcask solves this by making **all writes sequential appends**. The disk head never moves backwards. HDD sequential write speed is 100-200 MB/s — comparable to SSDs for sequential I/O.

```
Traditional DB write:   seek → write data → seek → update index → seek → WAL
                        ~30ms (3 seeks on HDD)

Bitcask write:          append to end of file + update in-memory map
                        ~microseconds (no seeks)
```

## How It Works

### Two Components

1. **Datafiles** — append-only log files on disk (the data)
2. **KeyDir** — in-memory hash map (the index): `key → {file_id, offset, size}`

### File Layout

Datafiles store entries back-to-back in binary format. No separators, no newlines:

```
[Entry1][Entry2][Entry3][Entry4]...
```

Each entry:

```
┌─────────┬───────────┬──────────┬────────────┬─────────┬─────────┐
│  CRC    │ Timestamp │ Key Size │ Value Size │   Key   │  Value  │
│ 4 bytes │  8 bytes  │ 4 bytes  │  4 bytes   │  var    │   var   │
└─────────┴───────────┴──────────┴────────────┴─────────┴─────────┘
         Fixed header: 20 bytes              Variable
```

- **CRC** — checksum of everything after it (integrity verification)
- **Timestamp** — when this entry was written
- **Key Size / Value Size** — tells you exactly how many bytes to read next
- **Key / Value** — the actual data

You know where one entry ends and the next begins because the sizes tell you. No delimiter ambiguity.

### Operations

**PUT (write):**
1. Create entry with CRC, timestamp, key, value
2. Append to the active datafile (one sequential write)
3. Update KeyDir: `key → {file_id, current_offset, entry_size}`
4. Done. One disk write, one memory update.

**GET (read):**
1. Lookup key in KeyDir → `{file_id, offset, size}`
2. Open the file, seek to offset, read `size` bytes
3. Verify CRC (data integrity check)
4. Return value

One disk seek, one read. If key doesn't exist in KeyDir → instant "not found" (no disk access).

**DELETE:**
1. Append a **tombstone** entry (key with empty value) to active file
2. Remove key from KeyDir
3. Old data remains on disk until compaction

**UPDATE:**
1. Same as PUT — append new entry, update KeyDir to point to it
2. Old entry becomes dead data (will be cleaned up during merge)

### Datafile Rotation

When the active file hits a size threshold (10MB in our implementation):
- Close it → becomes immutable (never written to again)
- Open a new active file for writes

Multiple datafiles accumulate over time:
```
data_000000.db  (immutable, old)
data_000001.db  (immutable, old)
data_000002.db  (active, open for writes)
```

### The Problem: Dead Data

Since we only append and never modify in place, old values and deleted keys pile up:

```
File: [name=v1][age=25][name=v2][name=v3][age=DELETE]
       dead    dead     dead    live     tombstone
```

KeyDir only points to the latest. Everything else is garbage consuming disk space.

### Merge & Compaction

A background process cleans up old (immutable) files:

1. For each key in KeyDir that points to an old file:
   - Read the entry from the old file
   - Write it to a new merged file
2. Skip everything else (old versions, tombstones) — they're dead
3. Update KeyDir to point to new offsets in the merged file
4. Delete old files

```
Before: data_000000.db (100MB, 60% dead) + data_000001.db (100MB, 40% dead)
After:  data_000003.db (80MB, 0% dead)
```

Compaction is also sequential — reads old files forward, writes new file forward. No random I/O.

### Hint Files (Fast Recovery)

Problem: if the process crashes, it must rebuild KeyDir by scanning ALL datafiles. Slow for large datasets.

Solution: after merge, write a **hint file** alongside the merged datafile. The hint file contains:
```
[timestamp | key_size | entry_size | offset | key]
```

It's the KeyDir entries for that file — without the values. Much smaller than the data file.

On startup:
- If hint file exists → read it to rebuild KeyDir (fast, no value scanning)
- If no hint file → scan the full data file (slower, but correct)

## Strengths

- **High write throughput** — sequential appends, no disk seeks
- **Predictable read latency** — always exactly one disk seek
- **Simple crash recovery** — append-only files can't have partial/corrupt mid-file writes
- **Easy backup** — just copy the directory
- **Low write amplification** — each write is written once (until compaction)

## Weakness

- **All keys must fit in RAM** — KeyDir holds every key in memory
- If you have billions of keys with long key names, you need a lot of RAM
- Solution: shard across multiple Bitcask instances (horizontal scaling)

## Where It's Used

- **Riak** — distributed database (each node runs a Bitcask instance)
- Inspired designs in **RocksDB**, **LevelDB**, **Kafka** (log-structured storage)
- Any write-heavy workload on commodity hardware (HDDs)

## Project Structure

```
bitcask/
├── main.go       # CLI interface for testing
├── entry.go      # Binary record format (encode/decode)
├── datafile.go   # File operations (append, read at offset, rotate)
├── keydir.go     # In-memory hash index
├── bitcask.go    # Core engine (put, get, delete, open, close, recover)
├── merge.go      # Compaction (merge old files, remove dead data)
├── hint.go       # Hint file read/write for fast recovery
├── go.mod
└── README.md
```

## Running

```bash
go run .
```

```
Bitcask opened. Keys in store: 0
Commands: put <key> <value> | get <key> | del <key> | merge | keys | quit
> put name jagdeep
OK
> put city delhi
OK
> get name
jagdeep
> put name updated
OK
> get name
updated
> del city
DELETED
> get city
error: key not found
> keys
2 keys: [name age]
> merge
Merge complete: 2 keys compacted into data/data_000003.db
> quit
```

Restart — data persists:
```
Bitcask opened. Keys in store: 2
> get name
updated
```

## How Each File Maps to the Design

| File | Role | Key Concept |
|------|------|-------------|
| `entry.go` | Binary record format | Fixed 20-byte header + variable key/value. CRC for integrity. |
| `datafile.go` | Append-only log file | Sequential writes, read-at-offset, rotation on size limit. |
| `keydir.go` | In-memory index | Hash map: key → {file, offset, size}. The only index in the system. |
| `bitcask.go` | Engine coordinator | Ties it all together: put/get/delete, file rotation, crash recovery. |
| `merge.go` | Compaction | Reads only live data from old files, writes to new file, deletes old. |
| `hint.go` | Fast recovery | Small file with just key + offset info (no values). Speeds up startup. |

## Key Design Decisions

1. **Binary format, not CSV/JSON** — compact, fast to parse, no delimiter ambiguity
2. **Tombstone for deletes** — append-only means we can't remove data; tombstone marks it dead
3. **CRC on every entry** — detect corruption on read (bit rot, partial writes)
4. **Single writer** — only one active file open for writes (mutex protected)
5. **Multiple readers** — old files are immutable, safe to read concurrently
6. **Merge only touches old files** — active file is never compacted (writes keep flowing)

## Comparison to Other Storage Engines

| | Bitcask | B-Tree (PostgreSQL) | LSM Tree (RocksDB) |
|--|---------|--------------------|--------------------|
| Write pattern | Sequential append | Random (page updates) | Sequential (memtable flush) |
| Write speed | Very fast | Moderate | Fast |
| Read speed | 1 seek (via hash map) | O(log n) seeks | Multiple levels to check |
| Space amplification | High (until compaction) | Low | Moderate |
| All keys in RAM? | Yes (required) | No | No (bloom filters help) |
| Best for | Write-heavy, fits-in-RAM keys | General purpose | Write-heavy, large keyspace |
