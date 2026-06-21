# Bloom Filter

A probabilistic data structure that answers: "Is this item in the set?"

- **"Definitely NOT in the set"** — 100% certain, guaranteed
- **"Probably in the set"** — might be wrong (false positive possible)

Never gives false negatives. If it says "no", it's always right.

## What Problem Does It Solve?

Checking membership in a very large set **without storing all the items** and **without hitting a database**.

**Real example:** You have 1 billion registered usernames in a database. A new user types "jagdeep123" — is it taken?

- **Without bloom filter:** every signup attempt → database query. At scale, millions of queries for names that don't even exist.
- **With bloom filter:** check the filter first (in-memory, microseconds):
  - Filter says "no" → definitely available, skip the DB query entirely
  - Filter says "yes" → might be taken, now hit the DB to confirm

Result: you eliminate most database lookups for non-existent items.

## How It Works

Two components:
1. **Bit array** — `m` bits, all zeros initially
2. **k hash functions** — each maps an item to a position in the bit array

### Insert

Hash the item with `k` different hash functions. Each gives a position. Set those bits to 1.

```
Insert("jagdeep"):
  hash1("jagdeep") % m → position 3  → set bit 3 = 1
  hash2("jagdeep") % m → position 7  → set bit 7 = 1
  hash3("jagdeep") % m → position 12 → set bit 12 = 1

Bit array: [0,0,0,1,0,0,0,1,0,0,0,0,1,0,0,0]
                 ^           ^           ^
```

### Check

Hash with the same k functions. Check if ALL those bits are 1.

```
Check("jagdeep"):
  positions: 3, 7, 12
  bit 3 = 1? ✓  bit 7 = 1? ✓  bit 12 = 1? ✓
  → "Probably in set"

Check("unknown"):
  positions: 2, 7, 9
  bit 2 = 1? ✗
  → "Definitely NOT in set" (one zero is enough)
```

### Why False Positives Happen

Multiple items can set the same bits. If "alice" and "bob" together set bits 3, 7, 12 — then checking "jagdeep" (which also hashes to 3, 7, 12) shows all bits as 1, even though "jagdeep" was never inserted.

More items → more bits set to 1 → higher chance of accidental collisions → higher false positive rate.

## Why Multiple Hash Functions?

With 1 hash function, a single collision = false positive. With k hash functions, ALL k positions must be collided — exponentially less likely.

```
1 hash function:
  "jagdeep" → bit 3
  "alice"   → bit 3 (collision!)
  Check "alice" → bit 3 set → false positive immediately

3 hash functions:
  "jagdeep" → bits 3, 7, 12
  "alice"   → bits 3, 9, 15
  Check "alice" → bit 3 set? yes. bit 9 set? NO → definitely not in set
```

One colliding bit doesn't fool you when you check multiple positions.

**The tradeoff:** more hash functions = lower false positive rate (up to a point), but slightly slower add/check.

## The Math

Optimal bit array size: `m = -(n × ln(p)) / (ln2)²`
Optimal hash count: `k = (m/n) × ln2`

Where:
- `n` = expected number of items
- `p` = desired false positive rate

Example: 100K items, 1% FP rate → m = 958,506 bits (~117 KB), k = 7 hash functions.

That's 117 KB to represent 100K items with 99% accuracy. Storing the actual items would take megabytes.

## Where It's Used

| System | Use Case |
|--------|----------|
| RocksDB / LevelDB | Skip reading SSTables that definitely don't contain a key |
| Google Chrome | Check URLs against a malware list without downloading the full list |
| Cassandra | Avoid disk reads for non-existent partition keys |
| Medium | Recommend articles user hasn't read (filter already-read articles) |
| Akamai / CDNs | "Is this content cached at this edge node?" |
| Bitcoin | SPV nodes verify transactions without full blockchain |
| Web crawlers | "Have I already visited this URL?" |
| Spam filters | "Is this email in the known spam list?" |

## Key Properties

| Property | Value |
|----------|-------|
| Space efficiency | Much less memory than storing actual items |
| False negatives | Never — "no" is always correct |
| False positives | Possible — tunable via size and hash count |
| Deletion | Not supported (use counting bloom filter for that) |
| Time complexity | O(k) for both add and check — constant regardless of items stored |

## Implementation Details

### Double Hashing Trick

Instead of k independent hash functions (expensive), we use the double hashing technique:

```
h(item, i) = h1(item) + i × h2(item)
```

Two base hash functions simulate k hash functions. Mathematically proven to have similar properties to k independent functions.

### Optimal Sizing

You provide: expected items + desired false positive rate. The filter calculates the optimal bit array size and hash count automatically.

```go
// Filter for 10000 items with 1% false positive rate
bf := New(10000, 0.01)
// Automatically calculates: m = 95851 bits, k = 7 hash functions
```

## Running

```bash
cd bloom-filter
go run .
```

### Basic Operations

```
> add hello
Added: hello
> add world
Added: world
> check hello
hello → PROBABLY IN SET (could be false positive)
> check pizza
pizza → DEFINITELY NOT IN SET
```

### Load a Word List

```
> load data/words.txt
Loaded 30 words from data/words.txt
> check apple
apple → PROBABLY IN SET (could be false positive)
> check zebra
zebra → DEFINITELY NOT IN SET
```

### Check Stats

```
> stats
Items added: 32
Bit array size: 95851 bits (11.70 KB)
Hash functions: 7
Estimated FP rate: 0.0000%
```

### Test False Positive Rate

```
> test
Tested 10000 non-existent keys:
  False positives: 0
  Actual FP rate: 0.00%
  Expected FP rate: 0.00%
```

With only 32 items in a filter sized for 10000, the FP rate is near zero. Load a larger dataset to see false positives appear.

### Using System Dictionary (if available)

```
> load /usr/share/dict/words
Loaded 99171 words
> stats
Estimated FP rate: 0.97%
> test
Tested 10000 non-existent keys:
  False positives: 95
  Actual FP rate: 0.95%
  Expected FP rate: 0.97%
```

Close to the 1% target — the math works.

## Project Structure

```
bloom-filter/
├── main.go       # CLI interface + FP rate test
├── bloom.go      # Core: bit array, add, check, optimal sizing, double hashing
├── data/
│   └── words.txt # Sample word list for testing
├── go.mod
└── README.md
```

## Comparison: Bloom Filter vs Alternatives

| | Bloom Filter | HashSet | Sorted Array + Binary Search |
|--|-------------|---------|------------------------------|
| Memory | O(m) bits — very small | O(n) — stores all items | O(n) — stores all items |
| Lookup | O(k) — constant | O(1) — constant | O(log n) |
| False positives | Yes (tunable) | No | No |
| Deletion | No | Yes | Yes |
| Best when | Set is huge, can tolerate FP, need space efficiency | Need exactness, set fits in memory | Need sorted access |
