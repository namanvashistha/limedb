<div align="center">
  <picture>
    <img alt="LimeDB logo" src="logo/LimeDB_Logo-horizontal.png" height="100">
  </picture>
</div>
<br>

## LimeDB

**LimeDB** is a **distributed key-value database** built with **Java 21** and **Spring Boot**.  
It implements core distributed systems concepts including **consistent hashing with virtual nodes**, **peer-to-peer routing**, **automatic failover**, **consensus protocols**, and **dynamic rebalancing** for horizontal scalability.

Built as a hands-on learning platform for distributed systems fundamentals, LimeDB features a complete **hash ring implementation** with performance optimization, comprehensive monitoring, and a clear evolution path from PostgreSQL persistence through gossip protocols to custom LSM tree storage engines.

---



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
```java
// Virtual nodes for better distribution (150 per physical node)
String targetNode = consistentHashRing.getNode(key);
if (targetNode.equals(currentNodeUrl)) handleLocally();
else forwardToNode(targetNode);

// Hash ring automatically handles:
// - Load balancing across nodes
// - Minimal data movement when nodes are added/removed
// - 360-degree visualization for debugging
```

---

## Project Structure

```
limedb/
├── app/src/main/java/org/limedb/
│   ├── App.java              # Main Spring Boot application
│   └── node/
│       ├── config/           # Database & RestTemplate configuration
│       └── core/
│           ├── controller/   # Node REST API (peer-to-peer)
│           ├── dto/          # Data transfer objects
│           ├── model/        # Entry (key-value) entity
│           ├── repository/   # JPA repositories
│           └── service/      # Node business logic & routing
└── README.md
```

---

## Quick Start

### Prerequisites
- **Java 21**
- **PostgreSQL 14+** running on localhost:5432
- **Databases created**: `limedb_node_1`, `limedb_node_2`, `limedb_node_3`

```bash
# Quick database setup
./setup-postgres.sh
```

### 1. Start Peer Nodes

```bash
# Terminal 1: Node 1
./gradlew bootRun --args='--server.port=7001 --node.id=1'

# Terminal 2: Node 2  
./gradlew bootRun --args='--server.port=7002 --node.id=2'

# Terminal 3: Node 3
./gradlew bootRun --args='--server.port=7003 --node.id=3'
```

### 2. Test the API (Connect to ANY node)

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
curl http://localhost:7001/cluster/state

# Check hash ring statistics and ranges
curl http://localhost:7001/cluster/ring
```

**Concurrent Load Testing:**
```bash
cd scripts
# Concurrent testing with multiple workers
python bulk_set.py
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

### Application Properties

```properties
# PostgreSQL connection
spring.datasource.username=limedb
spring.datasource.password=limedb
spring.datasource.host=localhost
spring.datasource.port=5432

# JPA Configuration  
spring.jpa.hibernate.ddl-auto=update
spring.jpa.database-platform=org.hibernate.dialect.PostgreSQLDialect

# Peer-to-Peer Configuration
node.id=1
node.peers=http://localhost:7001,http://localhost:7002,http://localhost:7003

# Consistent Hashing Configuration
node.routing.virtual-nodes=150
node.routing.hash-algorithm=MD5

# Logging Configuration
logging.file.name=logs/limedb-node-${server.port}.log
logging.level.org.limedb.node.service.NodeService=DEBUG
```

### Runtime Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `--server.port` | HTTP port for this node | `--server.port=7001` |
| `--node.id` | Node identifier (1-based) | `--node.id=1` |
| `--node.peers` | Comma-separated peer URLs | `--node.peers=http://localhost:7001,http://localhost:7002` |

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
| **Language** | Java 21 | Modern JVM with performance improvements |
| **Framework** | Spring Boot 3.5.6 | Web framework & dependency injection |
| **Database** | PostgreSQL 14+ | Persistent storage per node |
| **ORM** | JPA/Hibernate | Database mapping & operations |
| **Build** | Gradle | Build automation & dependency management |
| **Architecture** | Peer-to-Peer | Distributed system pattern |
| **Routing** | Consistent Hashing | Virtual nodes with MD5 hash ring |
| **Communication** | HTTP REST | Inter-node communication |


---

## � Testing

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

## Contributing

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** (`git commit -m 'Add amazing feature'`)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Inspiration

LimeDB draws inspiration from:
- **Redis** - Simple key-value API and operational ease
- **Cassandra** - Distributed architecture patterns
- **PostgreSQL** - Reliable ACID storage engine
- **Spring Boot** - Developer-friendly Java framework

---

##  Why LimeDB?

- **Fast to Deploy** - Single JAR, familiar Spring Boot setup
- **Horizontally Scalable** - Add nodes as you grow
- **Durable** - PostgreSQL persistence with ACID guarantees
- **Predictable** - Hash-based routing, same key → same node
- **Developer Friendly** - REST API, familiar tools
- **Extensible** - Clean architecture for future enhancements

**Ready to scale your key-value storage?** ⭐ Star the repo and get started!
