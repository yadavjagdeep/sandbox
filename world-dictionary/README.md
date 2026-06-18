# World Dictionary

A read-only dictionary service that serves 170K+ word definitions without a database. Data lives in a single custom binary file (`.wdict`) on blob storage (S3, local filesystem), and lookups are served via pointed reads using an in-memory index.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  .wdict File Layout                  │
├──────────┬──────────────────┬───────────────────────┤
│  Header  │  Index Section   │    Data Section       │
│ (64 bytes)│  (~3MB for 170K) │  (up to 1TB)         │
└──────────┴──────────────────┴───────────────────────┘
```

### Header (64 bytes, fixed)

| Field        | Type   | Size   | Description                          |
|--------------|--------|--------|--------------------------------------|
| Magic        | string | 5 bytes| File identifier: "WDICT"             |
| Version      | uint32 | 4 bytes| Data version (bumped on each update) |
| WordCount    | uint32 | 4 bytes| Total number of words                |
| IndexOffset  | uint64 | 8 bytes| Byte offset where index starts       |
| IndexLength  | uint64 | 8 bytes| Length of index section in bytes      |
| DataOffset   | uint64 | 8 bytes| Byte offset where data starts        |
| Reserved     | -      | 27 bytes| Future use                          |

### Index Section

CSV format stored after the header:

```
word,offset,length
apple,0,66
tiger,66,70
...
```

- `offset`: position relative to the start of the data section
- `length`: number of bytes for that row in the data section

On server boot, this section is loaded into an in-memory `map[string]IndexEntry` for O(1) lookups. For 170K words, this uses ~5-6MB of RAM.

### Data Section

Raw CSV rows:

```
apple,a round fruit with red or green skin and a whitish interior
tiger,a large wild cat with orange fur and black stripes native to Asia
...
```

Each row is accessed via a pointed read (seek to offset, read N bytes). The server never loads the full data section into memory.

## How It Works

### Read Path (API Server)

1. On boot: read `meta.json` to get the current `.wdict` file path
2. Read first 64 bytes → parse header → get index offset and length
3. Read index section → load into in-memory map
4. On request: `GET /words/:word`
   - Lookup word in map → get `{offset, length}`
   - Pointed read: seek to `DataOffset + offset`, read `length` bytes
   - Parse the row, return the meaning as JSON

### Write Path (Offline Indexer)

1. Reads a raw CSV file (`word,meaning` per line)
2. Deduplicates (last occurrence wins for repeated words)
3. Computes offsets for each entry
4. Writes the `.wdict` file: header → index → data

### Update Flow

1. Monthly: receive a changelog (new/updated/deleted words)
2. Offline job merges changelog with existing data → produces a new `.wdict` file
3. Upload new file to storage (e.g., `dictionary_v2.wdict`)
4. Update `meta.json` to point to the new file
5. Rolling deployment: new server instances boot with updated data, old instances drain

Old files remain on storage as backups. Rollback = revert `meta.json` + redeploy.

## Project Structure

```
world-dictionary/
├── main.go              # Entry point — boots server, loads index
├── header.go            # 64-byte header read/write
├── index.go             # In-memory index (map), load/serialize
├── store.go             # Storage interface + local filesystem impl
├── handler.go           # HTTP handler (GET /words/:word)
├── indexer/
│   └── main.go          # Offline job — builds .wdict from raw CSV
├── data/
│   ├── raw.csv          # Sample dictionary for testing
│   └── meta.json        # Pointer to current .wdict file
├── go.mod
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.22+

### Build

```bash
# Install dependencies
go mod tidy

# Build the indexer
go build -o indexer-bin ./indexer/

# Build the server
go build -o server-bin .
```

### Create the .wdict File

```bash
./indexer-bin data/raw.csv data/dictionary_v1.wdict
```

Output:
```
Built data/dictionary_v1.wdict: 8 words, index@64 (108 bytes), data@172 (470 bytes)
```

### Run the Server

```bash
./server-bin
```

Output:
```
Dictionary v1 — 8 words
Index loaded: 8 entries
⇨ http server started on [::]:8080
```

### Query

```bash
curl http://localhost:8080/words/tiger
```

Response:
```json
{
  "word": "tiger",
  "meaning": "a large wild cat with orange fur and black stripes native to Asia"
}
```

Unknown word:
```bash
curl http://localhost:8080/words/notaword
```

```json
{
  "error": "word not found"
}
```

## Design Decisions

### Why a custom binary format?

- Single file = portable (S3, GCS, Azure Blob, local disk) — no vendor lock-in on storage
- Header + index + data in one artifact — no external dependencies
- Index at the front (after header) means servers load it without seeking past 1TB of data

### Why uint64 for offsets?

- The file can be up to 1TB. `uint32` maxes at ~4GB (`2^32`). Offsets into a 1TB file require `uint64`.

### Why not a database?

- Read-only workload with monthly batch updates — no create/update via API
- 170K words with large meanings (up to 1TB total) — too large for RAM, too simple for a DB
- Pointed reads via offset are sufficient and fast

### Why not scan the file on every server boot?

- Scanning 1TB on boot would take 15-30 minutes per instance
- With multiple server instances, that's wasteful and slow
- Instead, the index is pre-built and embedded in the file — servers read ~3MB on boot and are ready in seconds

### Why store length in the index?

- S3/blob storage range reads require exact byte ranges (`Range: bytes=start-end`)
- You cannot "read until newline" on S3
- Length allows a single precise read per lookup

### Why keep newlines in the data section?

- Makes the indexer's job easier (scan line by line to build offsets)
- Keeps the file human-debuggable
- Overhead: 170KB of newlines in a 1TB file — negligible

### Why meta.json + rolling deployment for updates?

- Server instances are stateless and immutable after boot
- No hot-reload complexity, no background polling
- Old files remain as backups — rollback is trivial (revert meta.json + redeploy)
- Kubernetes/ECS handles health checks and gradual rollout
- Fail-safe: if new file is corrupt, new pods fail health check, rollout halts automatically, old pods keep serving

### How are duplicates handled?

- The indexer deduplicates during build — if a word appears multiple times in the CSV, the last occurrence wins
- This supports the update flow: append updated meanings to the CSV, indexer keeps only the latest

## Memory Footprint

| Component | Size |
|-----------|------|
| Index (170K words) | ~5-6MB RAM |
| Header | 64 bytes |
| Data section | NOT in memory (pointed reads from disk/S3) |

## Storage Cost (S3)

| File | Size | Monthly cost |
|------|------|-------------|
| dictionary.csv | ~1TB | ~$23 (S3 Standard) |
| .wdict file | ~1TB | ~$23 |
| Index overhead | ~3MB | negligible |


