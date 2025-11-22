<div align="center">
  <picture>
    <img alt="LimeDB logo" src="logo/LimeDB_Logo-horizontal.png" height="100">
  </picture>
</div>
<br>

## LimeDB

**LimeDB** is a **highly-scalable distributed key-value store**.

It operates in a peer-to-peer architecture where any node can act as both coordinator and storage, eliminating single points of failure.

Keys are consistently hashed across multiple nodes using virtual node topology for optimal load distribution and minimal data movement during cluster changes.

Built as a hands-on learning platform for distributed systems fundamentals, LimeDB currently implements peer-to-peer routing and complete hash ring topology. Planned features include automatic failover, gossip protocol, consensus protocols, dynamic rebalancing, and evolution from PostgreSQL persistence toward custom LSM tree storage engines.


## Roadmap

### **Phase 1 Complete:** Basic peer-to-peer key-value store
- [x] Hash-based routing with automatic request forwarding
- [x] API with GET/SET/DELETE operations
- [x] Cluster state monitoring endpoint
- [x] PostgreSQL as transitional storage backend
- [x] Concurrent testing capabilities with performance metrics

### **Phase 2 In-Progress:** Consistent Hashing & Performance Analysis
- [x] **Consistent Hashing**: Full hash ring implementation with virtual nodes
- [x] **Hash Ring Visualization**: 360-degree ranges for easy debugging
- [x] **Database Indexing**: Optimized key lookups with unique constraints
- [x] **Performance Analysis**: Load testing with 15K+ concurrent requests
- [x] **Connection Pool Analysis**: RestTemplate bottleneck identification
- [x] **Comprehensive Logging**: File-based logging with rotation policies
- [x] **Ring Statistics API**: Real-time hash ring monitoring and distribution metrics

### Phase 3: Production Readiness
- [ ] **Connection Pool Optimization**: Custom RestTemplate configuration for high concurrency
- [ ] **Binary Internode Communication**: Move beyond HTTP/REST with gRPC/protobuf
- [ ] **Gossip Protocol**: Node discovery, failure detection, cluster membership, and topology changes
- [ ] **Health Checks**: Automatic failover when nodes go down
- [ ] **Dynamic Node Addition/Removal**: Scale nodes up and down with automatic rebalancing
- [ ] **Key Migration & Rebalancing**: Move data when topology changes
- [ ] **Replication Factor Support**: Consecutive N nodes in the ring for replication
- [ ] **Circuit Breakers**: Fault tolerance patterns for inter-node communication

### Phase 4: Custom Storage Engine
- [ ] **LSM Trees**: Replace PostgreSQL with custom key-value storage
- [ ] **Memory-Mapped Files**: Direct file system control
- [ ] **Custom Serialization**: Optimized data formats
- [ ] **WAL Implementation**: Write-ahead logging from scratch
- [ ] **Compaction Strategies**: Background merge operations for LSM efficiency

### Phase 5: Advanced Features
- [ ] **Advanced Gossip Features**: Anti-entropy, vector clocks, conflict resolution
- [ ] **Compression**: Custom compression algorithms
- [ ] **Cache Layers**: Multi-level caching strategies
- [ ] **Transaction Support**: ACID across multiple nodes
- [ ] **Read Replicas**: Separate read and write workloads
- [ ] **Cross-Datacenter Replication**: Geographic distribution

---

## Architecture

```
         Client App
              |
              | Can connect to ANY node
              |
    +---------+---------+---------+
    |         |         |         |
    v         v         v         v
+-------+  +-------+  +-------+
| Node 1|  | Node 2|  | Node 3|  
|:7001  |  |:7002  |  |:7003  |  
|       |  |       |  |       |  
| Routes|<-| Routes|<-| Routes|  Each node can:
| to    |->| to    |->| to    |  - Handle requests locally
| peers |  | peers |  | peers |  - Route to correct peer
|       |  |       |  |       |  - No single point of failure
+-------+  +-------+  +-------+
|  DB   |  |  DB   |  |  DB   |
|node_1|  |node_2|  |node_3|
+-------+  +-------+  +-------+
```

**Routing Logic (Consistent Hashing):**  
```go
// Virtual nodes for better distribution across physical nodes
targetNode := ring.GetNode(key)
if targetNode == currentNodeUrl {
    handleLocally()
} else {
    forwardToNode(targetNode)
}

// Hash ring automatically handles:
// - Load balancing across nodes
// - Minimal data movement when nodes are added/removed
// - MD5-based consistent hashing
```

---

## Project Structure

```
limedb/
├── cmd/
│   └── server/
│       └── main.go           # Main application entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── node/
│   │   ├── service.go        # Node business logic & routing
│   │   └── store.go          # Storage interface
│   ├── ring/
│   │   └── ring.go           # Consistent hashing implementation
│   └── server/
│       └── server.go         # HTTP server & API handlers
├── tui/                      # Terminal UI for monitoring
├── proxmox/                  # Proxmox LXC deployment scripts
└── README.md
```

---

## Quick Start

### Prerequisites
- **Go 1.21+**
- **PostgreSQL 14+** running on localhost:5432
- **Database created**: `limedb`

```bash
# Quick database setup
./setup-postgres.sh
```

### 1. Build and Start Cluster

```bash
# Build the binary
go build -o build/limedb ./cmd/server/main.go

# Start a 3-node cluster
NUM_NODES=3 ./run_go_cluster.sh
```

### 2. Start Individual Nodes

```bash
# Terminal 1: Node 1
./build/limedb -server.port 7001 -node.url "http://localhost:7001" -node.peers "http://localhost:7001,http://localhost:7002,http://localhost:7003"

# Terminal 2: Node 2  
./build/limedb -server.port 7002 -node.url "http://localhost:7002" -node.peers "http://localhost:7001,http://localhost:7002,http://localhost:7003"

# Terminal 3: Node 3
./build/limedb -server.port 7003 -node.url "http://localhost:7003" -node.peers "http://localhost:7001,http://localhost:7002,http://localhost:7003"
```

### 3. Test the API (Connect to ANY node)

**Single Request:**
```bash
# Set a value (routes to appropriate node automatically)
curl -X POST http://localhost:7001/api/v1/set \
  -H "Content-Type: application/json" \
  -d '{"key": "user:123", "value": "John Doe"}'

# Get a value (can query any node)
curl http://localhost:7001/api/v1/get/user:123

# Delete a value (routes to correct node)
curl -X DELETE http://localhost:7001/api/v1/del/user:123

# Check cluster state (shows all active nodes)
curl http://localhost:7001/api/v1/cluster/state

# Health check
curl http://localhost:7001/health
```

**Concurrent Load Testing:**
```bash
cd scripts
# Concurrent testing with multiple workers
python bulk_set.py
```


---

## 🐳 Docker Deployment

### Container Specifications
- **Base Image**: Alpine Linux with Go binary
- **Size**: <50MB (ultra-lightweight)
- **Memory**: Low memory footprint (configurable)
- **Security**: Non-root user, minimal attack surface
- **Performance**: Native Go binary performance

### Development Setup
```bash
# Pull from Docker Hub
docker pull namanvashistha/limedb:latest

# Start single node
docker run -d -p 8484:8484 \
  -e NODE_URL="http://localhost:8484" \
  -e NODE_PEERS="http://localhost:8484" \
  -e DB_HOST=your-postgres-host \
  -e DB_USERNAME=limedb \
  -e DB_PASSWORD=your-password \
  namanvashistha/limedb:latest
```

### Production Cluster
```bash
# Multi-node cluster with docker-compose
docker-compose up -d

# With custom configuration
NODE_URLS="http://node1:8484,http://node2:8484,http://node3:8484" \
DB_PASSWORD=secure_password \
docker-compose up -d
```

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_URL` | Required | This node's URL (e.g., http://localhost:8484) |
| `NODE_PEERS` | Required | Comma-separated peer URLs |
| `SERVER_PORT` | `8484` | HTTP server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USERNAME` | `limedb` | Database username |
| `DB_PASSWORD` | `limedb` | Database password |

### Health Checks
```bash
# Container health
docker ps --format "table {{.Names}}\t{{.Status}}"

# Application health  
curl http://localhost:7001/actuator/health

# Cluster metrics
curl http://localhost:7001/cluster/state | jq
```

---

## API Reference

### Node API (Any Port - 7001, 7002, 7003)

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| `POST` | `/api/v1/set` | Store key-value pair | `{"key": "user:1", "value": "Alice"}` |
| `GET` | `/api/v1/get/{key}` | Retrieve value by key | `/api/v1/get/user:1` |
| `DELETE` | `/api/v1/del/{key}` | Delete key | `/api/v1/del/user:1` |
| `GET` | `/cluster/state` | Node cluster info | Shows node ID, peers, and status |
| `GET` | `/cluster/ring` | Hash ring statistics | Virtual nodes, ranges, 360-degree visualization |

### Peer-to-Peer Behavior

- **Connect to ANY node**: All nodes expose the same API
- **Automatic routing**: Requests are automatically forwarded to the correct node
- **No single point of failure**: If one node is down, use another
- **Transparent**: Client doesn't need to know which node has the data


---

## Configuration

### Command Line Arguments

```bash
# Required parameters
./limedb -server.port 8484 -node.url "http://localhost:8484" -node.peers "http://localhost:8484,http://peer:8484"

# Environment variables (alternative)
export SERVER_PORT=8484
export NODE_URL="http://localhost:8484"
export NODE_PEERS="http://localhost:8484,http://peer:8484"
export DB_HOST=localhost
export DB_PORT=5432
export DB_USERNAME=limedb
export DB_PASSWORD=limedb
./limedb
```

### Runtime Parameters

| Parameter | Description | Example | Required |
|-----------|-------------|---------|----------|
| `-server.port` | HTTP port for this node | `-server.port 8484` | ✅ |
| `-node.url` | This node's full URL | `-node.url "http://localhost:8484"` | ✅ |
| `-node.peers` | Comma-separated peer URLs | `-node.peers "http://host1:8484,http://host2:8484"` | ✅ |
| `-db.host` | PostgreSQL host | `-db.host localhost` | Optional |
| `-db.port` | PostgreSQL port | `-db.port 5432` | Optional |
| `-db.username` | Database username | `-db.username limedb` | Optional |
| `-db.password` | Database password | `-db.password secret` | Optional |

### Database Setup

**Automatic Setup:**
```bash
./setup-postgres.sh
```

**Manual Setup:**
```sql
-- Create databases for each node
CREATE DATABASE limedb_node_1;
CREATE DATABASE limedb_node_2;  
CREATE DATABASE limedb_node_3;

-- Create user (optional)
CREATE USER limedb WITH PASSWORD 'limedb';
GRANT ALL PRIVILEGES ON DATABASE limedb_node_1 TO limedb;
GRANT ALL PRIVILEGES ON DATABASE limedb_node_2 TO limedb;
GRANT ALL PRIVILEGES ON DATABASE limedb_node_3 TO limedb;
```


---


## Design Principles

- **Simplicity First** - Start simple, evolve to complex
- **Horizontal Scaling** - Add nodes to scale storage & throughput  
- **Predictable Routing** - Same key always goes to same node
- **Operational Simplicity** - Easy to deploy and monitor
- **Storage Evolution** - Start with PostgreSQL, evolve to custom engines

---

## 🛠️ Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Language** | Go 1.21+ | High-performance, statically typed language |
| **HTTP Server** | FastHTTP | Ultra-fast HTTP server and client |
| **Database** | PostgreSQL 14+ | Persistent storage with pgx driver |
| **Database Driver** | pgx/v5 | High-performance PostgreSQL driver |
| **Build** | Go toolchain | Native Go build system |
| **Architecture** | Peer-to-Peer | Distributed system pattern |
| **Routing** | Consistent Hashing | MD5-based hash ring with virtual nodes |
| **Communication** | HTTP REST | Inter-node communication |
| **Monitoring** | Textual TUI | Terminal-based cluster monitoring |


---

## 📊 TUI Client

LimeDB includes a Terminal User Interface for interactive cluster operations:

```bash
# Navigate to TUI directory
cd tui/

# Install dependencies (Python 3.8+)
pip install -r requirements.txt

# Run the TUI client with cluster URLs
python main.py --urls http://node1:8080,http://node2:8080,http://node3:8080

# Alternative: Run without arguments to be prompted
python main.py
```

The TUI provides:
- Real-time cluster health monitoring
- Interactive key-value operations
- Node status visualization
- Performance metrics dashboard
- Automatic load balancing across cluster nodes

---

## Testing

### Manual Testing
```bash
# Test hash routing - same key goes to same node
curl -X POST http://localhost:8080/api/v1/set -H "Content-Type: application/json" -d '{"key": "test1", "value": "node_test"}'
curl http://localhost:8080/api/v1/get/test1  # Should return "node_test"

# Test persistence - restart nodes and data should remain
curl -X POST http://localhost:8080/api/v1/set -H "Content-Type: application/json" -d '{"key": "persist", "value": "data"}'
# Restart node servers
curl http://localhost:8080/api/v1/get/persist  # Should still return "data"
```

### Health Monitoring
```bash
# Check coordinator health
curl http://localhost:8080/api/v1/health
# Returns: {"status":"healthy","type":"coordinator","nodeCount":3,"nodes":["http://localhost:7001","http://localhost:7002","http://localhost:7003"]}
```

---

## Automated Releases

LimeDB uses automated semantic versioning based on commit messages:

### Version Bumping
- **Patch** (`v1.0.0 → v1.0.1`): Default for bug fixes and small changes
- **Minor** (`v1.0.0 → v1.1.0`): Add `[minor]`, `feat:`, or `feature:` to commit message
- **Major** (`v1.0.0 → v2.0.0`): Add `[major]` or `breaking:` to commit message

### Examples
```bash
git commit -m "Fix memory leak in hash ring"              # → v1.0.1 (patch)
git commit -m "Add metrics endpoint [minor]"              # → v1.1.0 (minor)
git commit -m "breaking: change API response format"      # → v2.0.0 (major)
git commit -m "Update documentation [skip tag]"           # → no release
```

### Available Downloads
Each release automatically provides:
- **Binaries**: Linux, macOS, Windows (AMD64 & ARM64)
- **Docker Images**: `namanvashistha/limedb:latest` and `namanvashistha/limedb:v1.x.x`
- **Proxmox LXC**: One-line installation script

## Contributing

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** with [semantic commit messages](#automated-releases)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

---

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.

---

## Inspiration

LimeDB draws inspiration from:
- **Redis** - Simple key-value API and operational ease
- **Cassandra** - Distributed architecture patterns
- **PostgreSQL** - Reliable ACID storage engine
- **Go ecosystem** - High-performance, statically typed simplicity

---

##  Why LimeDB?

- **Fast to Deploy** - Single binary, no dependencies
- **High Performance** - Go's efficiency with FastHTTP server
- **Horizontally Scalable** - Add nodes as you grow
- **Durable** - PostgreSQL persistence with ACID guarantees
- **Predictable** - Hash-based routing, same key → same node
- **Developer Friendly** - REST API, familiar tools
- **Extensible** - Clean Go architecture for future enhancements

**Ready to scale your key-value storage?** ⭐ Star the repo and get started!

## Resources:
- https://hazelcast.com/foundations/distributed-computing/cap-theorem/
- https://www.julianbrowne.com/article/brewers-cap-theorem/
- https://www.julianbrowne.com/
- https://www.toptal.com/big-data/consistent-hashing
- https://www.digitalocean.com/community/tutorials/understanding-database-sharding
- https://docs.scylladb.com/manual/master/kb/gossip.html
