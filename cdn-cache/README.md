# cdn-cache

Local simulation of CDN caching using Nginx as a cache layer in front of a Go origin server.

## What is a CDN?

A Content Delivery Network is a distributed network of servers (edge servers) placed geographically close to users. Instead of every request traveling to your origin server (which might be in one region), the CDN serves cached content from the nearest edge location.

Real-world CDN flow:
```
User in Tokyo → DNS resolves to nearest edge (Tokyo edge server)
  → Cache HIT: returns cached content in ~5ms
  → Cache MISS: fetches from origin (US-East), caches it, returns to user
    Next user in Tokyo gets a HIT — no round trip to US-East
```

## User Journey: Request for https://cdn.yourapp.com/images/photo.jpg

```
┌──────────┐
│  User    │  Types URL or browser requests an image
│ (Tokyo)  │
└────┬─────┘
     │
     ▼
┌──────────────┐
│  DNS Lookup  │  "Where is cdn.yourapp.com?"
│              │  DNS sees user is in Tokyo
│              │  Returns IP of Tokyo edge server (not origin)
└────┬─────────┘
     │
     ▼
┌─────────────────────┐
│  CDN Edge Server    │  Tokyo edge receives the request
│  (Tokyo)            │
│                     │
│  Check local cache: │
│  ┌───────────────┐  │
│  │ Cache HIT?    │  │
│  └───┬───────┬───┘  │
│    YES│     NO│      │
│      │       │       │
│      ▼       ▼       │
│  Return   Forward    │
│  cached   to origin  │
│  content  ──────────────────┐
│  (~5ms)              │      │
└──────────────────────┘      │
                              ▼
                    ┌──────────────────┐
                    │  Origin Shield   │  (optional, intermediate cache)
                    │  Prevents all    │
                    │  edges hitting   │
                    │  origin at once  │
                    │                  │
                    │  Cache HIT?      │
                    │  YES → return    │
                    │  NO  → forward   │
                    └────────┬─────────┘
                             │
                             ▼
                    ┌──────────────────┐
                    │  Origin Server   │  Your actual server (US-East)
                    │  (S3 / API)      │  Serves the image
                    └────────┬─────────┘
                             │
                    Response flows back:
                             │
                    Origin → Origin Shield (caches it)
                             │
                    Origin Shield → Tokyo Edge (caches it)
                             │
                    Tokyo Edge → User (~200ms first time)
                             │
                    Next request from Tokyo:
                    Edge has it cached → returns in ~5ms
                    Origin never touched again until TTL expires
```

## Why Use a CDN?

- Latency: serving from a nearby edge is faster than crossing continents to the origin
- Origin offloading: 90%+ of requests served from cache, origin handles very little traffic
- Bandwidth: CDN absorbs traffic spikes (product launches, viral content)
- Availability: if origin goes down, cached content is still served
- Static assets: images, CSS, JS, videos — perfect for caching (rarely change)
- API responses: product catalogs, config data — cacheable if not user-specific

## What a Real CDN Setup Looks Like

```
User (browser)
  → DNS (routes to nearest CDN edge based on user's geolocation)
    → CDN Edge Server (checks local cache)
      → Cache HIT: returns immediately
      → Cache MISS:
        → Origin Shield (optional intermediate cache — prevents all edges hitting origin)
          → Origin Server (your API / S3 bucket)
            → Returns response
          ← Cached at origin shield
        ← Cached at edge
      ← Returns to user
```

Real CDN providers: CloudFront (AWS), Cloudflare, Akamai, Fastly.

In production you'd:
- Put static assets in S3, front it with CloudFront
- Configure TTLs, cache invalidation rules, custom headers
- Use a custom domain (cdn.yourapp.com) with SSL
- Set cache-control headers on the origin to tell the CDN how long to cache

## What We Did Instead (Local Simulation)

We can't simulate geographic distribution locally, but we can simulate the caching behavior — which is the core of what a CDN does.

```
curl → Nginx (:8080, acts as CDN edge) → Go server (:3000, acts as origin)
       Caches responses on disk            Serves images + API data
       Returns X-Cache-Status header       Prints "ORIGIN HIT" when accessed
```

- Nginx acts as the CDN edge — caches responses, serves from cache on subsequent requests
- Go server acts as the origin — serves images and API data
- Custom local domains: cdn.local (Nginx) and api.local (Go server)
- X-Cache-Status header shows HIT/MISS — proves caching is working
- Go server logs "ORIGIN HIT" only on cache misses — proves origin is bypassed on hits

What this covers:
- Cache HIT vs MISS behavior
- TTL-based cache expiry
- Cache headers (X-Cache-Status, Cache-Control, max-age)
- Origin offloading (origin only hit on first request)
- Caching both static files (images) and API responses

What this doesn't cover:
- Geographic distribution and DNS-based routing
- Edge locations worldwide
- Origin shield (intermediate cache layer)
- Cache invalidation via API (CloudFront has invalidation APIs)
- SSL/TLS termination at edge

## Architecture

```
/etc/hosts:  127.0.0.1 cdn.local api.local

cdn.local:8080 (Nginx in Docker)
  ├── /images/*     → cache + proxy to origin
  └── /api/*        → cache + proxy to origin

api.local:3000 (Go server on host)
  ├── /images/:name → serves image files from disk
  └── /api/products → returns product JSON
```

## Setup

Add local domains:
```bash
sudo sh -c 'echo "127.0.0.1 cdn.local api.local" >> /etc/hosts'
```

Start Nginx (CDN edge):
```bash
cd sandbox/cdn-cache
sudo docker-compose up -d
```

Start Go origin server:
```bash
go run main.go
```

## Test

```bash
# First request — MISS (origin prints "ORIGIN HIT")
curl -I http://cdn.local:8080/images/sample.jpeg
# X-Cache-Status: MISS

# Second request — HIT (origin stays silent)
curl -I http://cdn.local:8080/images/sample.jpeg
# X-Cache-Status: HIT

# API caching works too
curl -I http://cdn.local:8080/api/products
# X-Cache-Status: MISS
curl -I http://cdn.local:8080/api/products
# X-Cache-Status: HIT
```

Use `-I` to see headers (X-Cache-Status). Use `-v` to see both headers and body.

## Nginx Cache Config Explained

```nginx
proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=cdn_cache:10m max_size=100m inactive=10m;
```
- Cached files stored at /var/cache/nginx inside the container
- keys_zone: 10MB of shared memory for cache keys
- max_size: 100MB max disk usage for cached content
- inactive: remove cached items not accessed for 10 minutes

```nginx
proxy_cache_valid 200 5m;
```
- Cache successful (200) responses for 5 minutes (TTL)

```nginx
add_header X-Cache-Status $upstream_cache_status;
```
- Adds HIT/MISS/EXPIRED header to every response

```nginx
proxy_pass http://host.docker.internal:3000;
```
- On cache MISS, forward request to Go origin server on host machine

## Cache Lifecycle

```
Request 1: MISS → fetch from origin → store in cache → return to client
Request 2: HIT → serve from cache → origin never touched
...
After 5 minutes (TTL expires):
Request N: EXPIRED → fetch fresh from origin → update cache → return to client
```

## When to Cache at CDN vs Not

Cache (serve from edge):
- Static assets: images, CSS, JS, fonts, videos
- Public API responses: product listings, config, public profiles
- Content that doesn't change per user

Don't cache (always hit origin):
- Authenticated/personalized responses (user dashboard, cart)
- Real-time data (stock prices, live scores)
- POST/PUT/DELETE requests (writes should always reach origin)
