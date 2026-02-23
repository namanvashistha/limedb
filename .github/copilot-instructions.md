# LimeDB Development Guide

## Architecture Overview

**LimeDB** is a peer-to-peer distributed key-value store with no primary/leader concept. Every node can handle client requests and route to the correct node using consistent hashing with virtual nodes.

### Core Components

- **[internal/ring/ring.go](../internal/ring/ring.go)**: Consistent hash ring with MD5 hashing and virtual nodes for load distribution
- **[internal/node/service.go](../internal/node/service.go)**: Request routing logic - handles local operations or forwards to target node via FastHTTP
- **[internal/gossiper/gossiper.go](../internal/gossiper/gossiper.go)**: Three-phase gossip protocol (SYN → ACK → ACK2) for cluster membership and failure detection
- **[internal/server/server.go](../internal/server/server.go)**: FastHTTP-based HTTP server with OpenTelemetry tracing middleware
- **[internal/node/store.go](../internal/node/store.go)**: Thread-safe in-memory storage using `sync.Map`

### Request Flow

```
Client → Any Node → Hash Ring Lookup → Local Store OR Forward to Target Node
```

Each node includes itself in the ring, enabling single-node clusters. The gossiper maintains cluster membership and syncs with the ring every 5 seconds (see [cmd/server/main.go:85](../cmd/server/main.go)).

## Critical Patterns

### 1. Consistent Hashing
Keys are hashed (MD5) and mapped to virtual nodes on a 64-bit ring. Nodes are added with multiple virtual nodes (default: 256) for better distribution. See [internal/ring/ring.go:40-50](../internal/ring/ring.go).

### 2. Request Forwarding
When `ring.GetNode(key) != currentNodeUrl`, requests forward to the target node using FastHTTP's `AcquireRequest/ReleaseRequest` pattern for zero-allocation HTTP calls. See [internal/node/service.go:154-167](../internal/node/service.go).

### 3. Gossip Protocol
- **GOSSIP_SYN**: Node sends heartbeat digests to random peer
- **GOSSIP_ACK**: Peer responds with updates and requests
- **GOSSIP_ACK2**: Original node sends requested updates
- Peer states: `active` (recent heartbeat), `stale` (old heartbeat), `dead` (exceeded threshold)

Only `active` and `stale` peers are kept in the hash ring. See [cmd/server/main.go:87-104](../cmd/server/main.go).

### 4. OpenTelemetry Integration
All HTTP requests are automatically traced with context propagation. Custom `fasthttpHeaderCarrier` adapts FastHTTP to OTel's `TextMapCarrier`. Metrics include request count and latency histograms. See [internal/server/server.go:73-121](../internal/server/server.go).

## Development Workflow

### Building & Running

```bash
# Build binary
go build -o build/limedb ./cmd/server/main.go

# Run local 5-node cluster
NUM_NODES=5 bash run_go_cluster.sh

# Run single node
./build/limedb -server.port 8484 -node.url http://localhost:8484 -otel.endpoint ""
```

### Configuration Flags
- `-server.port`: HTTP server port (default: 8484)
- `-node.url`: **REQUIRED** - This node's URL (must match server port)
- `-node.peers`: Comma-separated peer URLs (for cluster mode)
- `-node.routing.virtual-nodes`: Virtual nodes per physical node (default: 256)
- `-otel.endpoint`: OTLP collector endpoint (empty string disables telemetry)

See [internal/config/config.go:19-35](../internal/config/config.go).

### Web Dashboard

```bash
cd web
npm install
npm run dev  # http://localhost:3000
```

Set `LIMEDB_SEED_URL` env var to connect to cluster. Dashboard uses Next.js API proxy at `/api/proxy` to bypass CORS. See [web/README.md:38-56](../web/README.md).

## Project-Specific Conventions

### 1. FastHTTP Usage
Uses [valyala/fasthttp](https://github.com/valyala/fasthttp) instead of `net/http` for performance. Always acquire/release requests for zero-allocation:

```go
req := fasthttp.AcquireRequest()
resp := fasthttp.AcquireResponse()
defer fasthttp.ReleaseRequest(req)
defer fasthttp.ReleaseResponse(resp)
```

### 2. Structured Logging
Uses [internal/logger/logger.go](../internal/logger/logger.go) wrapper around OTel logging with key-value pairs:

```go
logger.Info("GET request", "key", key, "client.ip", ctx.RemoteIP().String())
```

### 3. No Self-Node Priority
All nodes are equal peers. The node running code has no special treatment. Node lists are sorted alphabetically by URL. See [web/README.md:8-10](../web/README.md).

### 4. In-Memory Storage
Currently uses `sync.Map` for storage. Phase 4 roadmap includes LSM tree implementation to replace this. See [README.md:48-53](../README.md).

## Common Tasks

### Adding API Endpoints
1. Add route pattern in [internal/server/server.go:144-170](../internal/server/server.go) router switch
2. Implement handler method in same file
3. Use OTel `tracedCtx` from `ctx.UserValue("tracedCtx")` for span propagation

### Modifying Gossip Logic
- Message types defined in [internal/gossiper/message_types.go](../internal/gossiper/message_types.go)
- Heartbeat thresholds: `active` (<2 heartbeats behind), `stale` (2-5 behind), `dead` (>5 behind)
- Default gossip interval: 2 seconds (see [internal/gossiper/gossiper.go:480](../internal/gossiper/gossiper.go))

### Testing Load
Use provided Python scripts:

```bash
cd scripts
uv run hulk.py http://localhost:7001 --requests 15000 --concurrency 50
```

## Deployment

### Proxmox LXC (Production)
```bash
# Configure Grafana Cloud credentials
bash proxmox/configure-grafana.sh

# Deploy container with OTel collector
bash proxmox/limedb_lxc.sh
```

Includes systemd services for `limedb` and `otel-collector`. See [OTEL_INTEGRATION.md](../OTEL_INTEGRATION.md).

## Anti-Patterns to Avoid

- ❌ Don't use `net/http` - stick with FastHTTP for consistency
- ❌ Don't treat current node specially - all nodes are equal
- ❌ Don't modify ring topology directly - sync through gossiper
- ❌ Don't log plain strings - always use structured key-value logging
- ❌ Don't assume PostgreSQL - storage is in-memory for now (see roadmap)
