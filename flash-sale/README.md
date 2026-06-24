# Flash Sale

A flash sale system that handles millions of concurrent purchase attempts for limited inventory — without overselling. Think Flipkart Big Billion Days, Amazon Lightning Deals, or IRCTC Tatkal.

## The Problem

1000 iPhones go on sale at 12:00 PM. 1 million users click "Buy Now" simultaneously. How do you:
- Not oversell (exactly 1000 solvd, not 1001)
- Not crash under load
- Give users a fast response ("got it" or "sold out")

## The Key Insight

**Don't let 1M requests reach your database.** If you have 1000 items, only ~1000 requests should get through. Reject the rest instantly at the edge.

99.9% of requests get an instant "sold out" without ever touching the database.

"Rejecting requests cheaply and quickly is just as important as processing successful orders."

## Architecture

```
┌──────────────┐     ┌─────────────────────────┐     ┌───────────┐     ┌────────────┐
│ 1M Users     │────→│ Redis Gate (DECR)        │────→│ API Server│────→│ PostgreSQL │
│ click "Buy"  │     │ slots: 1000 → 999 → ... │     │           │     │ inventory  │
└──────────────┘     └─────────────────────────┘     └───────────┘     └────────────┘
                              ↓ (slots = 0)
                     ┌─────────────────────────┐
                     │ 999,000 instant "sold out"│
                     │ (never hit DB)            │
                     └─────────────────────────┘

Background:
┌──────────────┐
│  Releaser    │ every 30s: held > 10 min && not paid → release + INCR gate
└──────────────┘
```

## Design Decisions

### 1. Redis Gate (Traffic Control)

**Problem:** 1M requests hitting DB = DB dies (connection limit ~500).

**Solution:** Redis counter starts at inventory count (1000). Each buy attempt does atomic `DECR`:
- Value >= 0 → allowed through to API/DB
- Value < 0 → instant "sold out" (INCR back to restore counter)

**Why Redis:** `DECR` is atomic, single-threaded, handles millions/sec. One command to decide pass/reject. The "1M users" problem becomes a "1000 users" problem at the DB level.

### 2. Pre-Created Inventory Rows (One Row Per Unit)

```sql
-- 1000 iPhones = 1000 rows
inventory:
  id: uuid-1, item_id: "iphone", user_id: NULL  ← available
  id: uuid-2, item_id: "iphone", user_id: NULL  ← available
  ...
```

**Why not a single counter row?** If all 1000 requests try to `UPDATE count = count - 1` on one row, they serialize (row-level lock). With 1000 separate rows + `SKIP LOCKED`, each request gets a different row — no contention.

### 3. FOR UPDATE SKIP LOCKED (No Overselling)

```sql
SELECT id FROM inventory
WHERE item_id = 'iphone' AND user_id IS NULL
LIMIT 1
FOR UPDATE SKIP LOCKED;

UPDATE inventory SET user_id = 'user_123', held_at = NOW()
WHERE id = <selected_id>;
```

- `FOR UPDATE`: locks the row atomically
- `SKIP LOCKED`: if another user locked it, skip (don't wait)
- Each concurrent user gets a different available row
- No overselling, no contention, no deadlocks

### 4. Hold with 10-Minute Timeout

User claims an item → has 10 minutes to complete payment.
- Paid within 10 min → `paid = TRUE`, purchase confirmed
- Didn't pay → background job releases hold (`user_id = NULL`) + increments Redis gate

**Why hold?** User needs time to enter payment details, confirm address, etc. Without a hold, someone else could snatch the item while they're typing their card number.

**Why release after timeout?** Prevents inventory being locked forever by abandoned checkouts. Released items go back into the pool for other users.

### 5. Releaser (Background Job)

Runs every 30 seconds. Finds items where:
- `held_at` older than 10 minutes
- `paid = FALSE`

Releases them (user_id = NULL) AND increments the Redis gate counter — so new users can pass through the gate and claim the released items.

### 6. No Dedup / No One-Per-User Limit

We don't restrict one item per user. The gate is the only control — once slots hit 0, nobody gets in. If a user passes the gate multiple times, they can claim multiple items. Business can add this constraint if needed.

### 7. Why PostgreSQL Handles the DB Load Fine

After the Redis gate, only ~1000 requests reach the DB. Each is a tiny transaction (one SELECT + one UPDATE). PostgreSQL handles 1000 concurrent small transactions trivially. The gate eliminated the scaling problem.

## Where This Pattern Applies

This is not a flash-sale-specific pattern. It's a **limited inventory + concurrent demand** pattern that appears everywhere:

| System | Inventory | Gate needed? | Hold timeout? |
|--------|-----------|-------------|---------------|
| Flash sale (Flipkart/Amazon) | 1000 iPhones | Yes (extreme spike) | 10 min |
| IRCTC Tatkal | Few Tatkal seats | Yes (10 AM spike) | 10 min |
| Normal IRCTC booking | Hundreds of seats | No (spread traffic) | 15 min |
| BookMyShow (movie tickets) | 200 seats per show | Maybe (new Marvel release) | 8 min |
| Concert tickets (Coldplay) | 50K seats | Yes (massive demand) | 10 min |
| Regular e-commerce (Amazon) | Large inventory | No (no spike) | Hours (cart) |
| Grocery delivery (Zepto/Blinkit) | 5 bottles of milk in dark store | No (moderate contention) | Until picker confirms |
| Hotel booking (OYO/Booking.com) | 1 room per date | No (spread traffic) | 15-30 min |
| Flight booking (MakeMyTrip) | Finite seats per flight | No (spread traffic) | 20 min |

**The core pattern is always:**
1. **Gate** (optional) — control how many enter (only for extreme spikes)
2. **Claim** — atomic lock on a specific inventory unit
3. **Hold** — time-limited reservation for payment
4. **Release** — timeout → item back to pool

The gate is a dial you turn up when contention is extreme. Remove it and the rest still works — just can't handle millions of simultaneous requests.

## API

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/sale/items` | List items with remaining stock |
| POST | `/sale/items/:id/buy` | Attempt to purchase (gate + claim) |
| POST | `/sale/items/:id/pay` | Confirm payment for held item |

### POST /sale/items/iphone/buy
```json
// Request
{"user_id": "user_123"}

// Success (got one!)
{"status": "held", "inventory_id": "uuid-42", "message": "complete payment within 10 minutes"}

// Sold out
{"error": "sold out"}
```

### POST /sale/items/iphone/pay
```json
// Request
{"user_id": "user_123", "inventory_id": "uuid-42"}

// Success
{"status": "paid", "message": "purchase confirmed!"}
```

## Data Model

```sql
CREATE TABLE inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id VARCHAR(50) NOT NULL,
    user_id VARCHAR(50),           -- NULL = available
    held_at TIMESTAMP,             -- when claimed
    paid BOOLEAN DEFAULT FALSE     -- payment confirmed?
);
```

**State derived from columns:**

| user_id | paid | State |
|---------|------|-------|
| NULL | FALSE | Available |
| set | FALSE | Held (10 min timer running) |
| set | TRUE | Sold (done) |

**Indexes:**
```sql
CREATE INDEX idx_inventory_available ON inventory(item_id) WHERE user_id IS NULL;
CREATE INDEX idx_inventory_held ON inventory(held_at) WHERE user_id IS NOT NULL AND paid = FALSE;
```

Partial indexes — only index rows that matter for each query. Stay small regardless of total data.

## Back-of-Envelope

```
Flash sale event:
  - 1M users hit /buy at 12:00:00
  - Redis DECR: handles 1M ops in <1 second
  - 1000 pass gate → DB handles 1000 small txns in ~100ms
  - 999,000 get instant "sold out" (never touch DB)
  - Total time to sell out: <2 seconds
```

## Running Locally

### Start Infrastructure

```bash
sudo docker-compose up -d
```

### Run the API

```bash
go run .
```

Output:
```
Gate initialized: iphone = 10 slots
Flash sale API on :8080
```

### Run the Releaser (separate terminal)

```bash
go run ./releaser/
```

### Test: Check Stock

```bash
curl http://localhost:8080/sale/items
```

### Test: Buy an Item

```bash
curl -X POST http://localhost:8080/sale/items/iphone/buy \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_1"}'
```

Response:
```json
{"status": "held", "inventory_id": "uuid-42", "message": "complete payment within 10 minutes"}
```

### Test: Pay

```bash
curl -X POST http://localhost:8080/sale/items/iphone/pay \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_1", "inventory_id": "<uuid from buy response>"}'
```

### Test: Sell Out

```bash
for i in $(seq 1 10); do
  curl -s -X POST http://localhost:8080/sale/items/iphone/buy \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"user_$i\"}"
done

# 11th attempt
curl -X POST http://localhost:8080/sale/items/iphone/buy \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_11"}'
# Response: {"error": "sold out"}
```

### Test: Release After Timeout

Wait 10 minutes (or temporarily change HoldDuration in releaser to 30 seconds for testing). Items that weren't paid get released and become available again.

## Project Structure

```
flash-sale/
├── main.go              # API (buy + pay)
├── gate/redis.go        # Redis gate: DECR/INCR slot management
├── store/postgres.go    # DB: claim (SKIP LOCKED), pay, release
├── releaser/main.go     # Background: release expired holds + restore gate
├── migration/schema.sql # Inventory table + seed data + indexes
├── docker-compose.yaml  # PostgreSQL + Redis
└── README.md
```

## What's NOT Implemented (Production)

| Feature | Description |
|---------|-------------|
| Virtual queue / waiting room | "You are #4523 in line" — reduces perceived unfairness |
| Sale start time enforcement | Lock /buy endpoint until sale time via config |
| Rate limiting per IP | Prevent bots from hammering the gate |
| Payment gateway integration | Actual Razorpay/Stripe call |
| Seat/unit selection | User picks specific item (not "any available") |
| Notifications | "Your hold expires in 2 minutes" push notification |
| Analytics | Conversion rate, gate pass rate, payment drop-off |
| Predictive scaling | Pre-scale API servers before sale based on waitlist count |

## Cleanup

```bash
sudo docker-compose down -v
```
