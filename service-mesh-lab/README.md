# Service Mesh Lab

Hands-on exploration of Envoy sidecar proxy and Istio service mesh using a Go-based ID generator service.

---

## Key Concepts

### Envoy
A high-performance L4/L7 proxy written in C++. It handles traffic interception, load balancing, retries, timeouts, circuit breaking, and observability — all without your app knowing it exists. It's the data plane component that actually processes network traffic.

### Istio
A service mesh platform that manages communication between microservices. It provides traffic management, security (mTLS, RBAC), and observability. Istio uses Envoy as its data plane proxy.

### Sidecar Pattern
A deployment pattern where a proxy (Envoy) runs alongside your application in the same pod. All inbound/outbound traffic is forced through the proxy via iptables rules. Your app talks plain HTTP to localhost; the sidecar handles encryption, auth, retries, etc.

### istiod
Istio's control plane. A single binary that:
- Watches Kubernetes for changes (new pods, services, policies)
- Translates Istio resources (AuthorizationPolicy, VirtualService) into Envoy config
- Pushes config to all sidecars via xDS (gRPC streaming protocol)
- Acts as a Certificate Authority (issues mTLS certificates to pods)

### mTLS (Mutual TLS)
Both sides of a connection verify each other's identity using certificates. In the mesh, every pod gets a SPIFFE identity certificate from istiod. Sidecars use these to encrypt all pod-to-pod traffic automatically. Apps don't manage any TLS — it's invisible to them.

### ext_authz (External Authorization)
An Envoy filter that calls an external service to make allow/deny decisions before forwarding a request. Used for centralized RBAC — the auth service validates tokens, checks roles, and returns allow/deny. Envoy enforces the decision.

### xDS Protocol
The API istiod uses to push configuration to Envoy proxies. It's a set of gRPC streaming APIs (EDS for endpoints, CDS for clusters, LDS for listeners, RDS for routes). Proxies maintain a persistent connection to istiod and receive updates in real-time.

---

## Project Structure

```
service-mesh-lab/
├── README.md
├── docker-compose.yaml          # Stage 1: standalone Envoy setup
├── envoy/
│   └── envoy.yaml               # Manual Envoy config (Stage 1)
├── services/
│   ├── id-generator/            # Go service — generates unique IDs
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   ├── go.mod / go.sum
│   │   ├── snowflake/           # Twitter Snowflake ID implementation
│   │   └── central/             # Central batching ID implementation
│   ├── auth-service/            # Simple auth service (simulates ext_authz)
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── go.mod
│   └── caller/                  # Service that calls id-generator (for mesh testing)
│       ├── Dockerfile
│       ├── main.go
│       └── go.mod
└── k8s/                         # Kubernetes manifests (Stage 2)
    ├── id-generator.yaml        # Deployment + Service
    └── caller.yaml              # Deployment + Service
```

---

## Stage 1: Standalone Envoy (Docker Compose)

**Goal:** Understand Envoy config from scratch — listeners, filters, routes, clusters.

### Architecture

```
Client (curl, port 10000)
    → Envoy proxy
        → ext_authz filter → auth-service (port 9000)
        → router filter → id-generator (port 8080)
```

### What We Proved

- Envoy intercepts traffic on port 10000 and forwards to id-generator on 8080
- ext_authz calls auth-service before forwarding — denied requests never reach the app
- Auth-service injects `x-user-id` header — app receives identity without doing auth
- Retries: Envoy silently retries 5xx responses (up to 2 retries)
- Timeouts: Envoy returns 504 if upstream doesn't respond within 3 seconds

### Run

```bash
cd service-mesh-lab
docker-compose up --build
```

### Test

```bash
# No token → 403 (blocked by auth-service)
curl localhost:10000/snowflake

# Valid token → 200 (auth passes, forwarded to id-generator)
curl localhost:10000/snowflake -H "Authorization: Bearer my-secret-token"

# See injected headers
curl localhost:10000/debug/headers -H "Authorization: Bearer my-secret-token" | jq .

# Timeout test (returns 504 after 3s)
curl localhost:10000/slow -H "Authorization: Bearer my-secret-token"

# Retry test (Envoy retries on 500, client often gets 200)
curl localhost:10000/flaky -H "Authorization: Bearer my-secret-token"
```

---

## Stage 2: Kubernetes + Istio Sidecar Mode (minikube)

**Goal:** See how Istio automates everything we did manually in Stage 1.

### Architecture

```
caller pod (2/2)                    id-generator pod (2/2)
┌─────────────────┐               ┌─────────────────┐
│ caller app      │               │ id-generator app│
│ (port 8081)     │               │ (port 8080)     │
│       │         │               │       ▲         │
│       ▼         │               │       │         │
│ istio-proxy     │───── mTLS ───▶│ istio-proxy     │
│ (Envoy sidecar) │               │ (Envoy sidecar) │
└─────────────────┘               └─────────────────┘
         ▲                                 ▲
         │          xDS (config push)      │
         └──────────── istiod ─────────────┘
```

### What We Proved

- Sidecar injection: deploy 1 container, get 2 (app + istio-proxy) automatically
- istiod generates full Envoy config (listeners, routes, clusters) without any manual envoy.yaml
- Service discovery: sidecar knows about all services in the cluster automatically
- mTLS: caller → id-generator traffic is encrypted, neither app handles TLS
- `istioctl proxy-config` shows the auto-generated Envoy config

### Setup

```bash
# Start minikube
sudo minikube start --memory=8192 --cpus=4 --driver=docker --force

# Install Istio
sudo istioctl install --set profile=demo -y

# Enable sidecar injection
sudo kubectl label namespace default istio-injection=enabled

# Build and load images
cd services/id-generator && sudo docker build -t id-generator:latest .
sudo minikube image load id-generator:latest

cd ../caller && sudo docker build -t caller:latest .
sudo minikube image load caller:latest

# Deploy
sudo kubectl apply -f k8s/id-generator.yaml
sudo kubectl apply -f k8s/caller.yaml
```

### Test

```bash
# Verify pods have sidecars (2/2)
sudo kubectl get pods

# Call id-generator directly
sudo kubectl exec -it deploy/id-generator -c id-generator -- wget -qO- http://localhost:8080/snowflake

# Call id-generator through the mesh (caller → sidecar → mTLS → sidecar → id-generator)
sudo kubectl exec -it deploy/caller -c caller -- wget -qO- http://localhost:8081/call-id-generator
```

### Inspect Envoy Config (auto-generated by istiod)

```bash
# Listeners (what ports Envoy listens on)
sudo istioctl proxy-config listeners deploy/id-generator

# Routes (path → cluster mapping)
sudo istioctl proxy-config routes deploy/id-generator

# Clusters (backend services)
sudo istioctl proxy-config clusters deploy/id-generator

# Certificates (mTLS identity)
sudo istioctl proxy-config secret deploy/id-generator
```

---

## Stage 3: Ambient Mode (TODO)

Remove sidecars, use ztunnel (per-node L4 proxy) + waypoint proxy (per-namespace L7 proxy). Same security and routing, less resource overhead.

---

## Key Differences Between Stages

| | Stage 1 | Stage 2 | Stage 3 (TODO) |
|---|---|---|---|
| Envoy config | Written by hand | Generated by istiod | Generated by istiod |
| Proxy location | Separate container | Sidecar in pod | ztunnel on node + waypoint |
| mTLS | Not configured | Automatic | Automatic |
| Auth | ext_authz in envoy.yaml | AuthorizationPolicy resource | AuthorizationPolicy resource |
| Service discovery | Manual (Docker DNS) | Automatic (Kubernetes API) | Automatic (Kubernetes API) |

---

## Cleanup

```bash
# Stop minikube (frees resources, keeps state)
sudo minikube stop

# Delete everything (full cleanup)
sudo minikube delete

# Stop Stage 1
docker-compose down
```
