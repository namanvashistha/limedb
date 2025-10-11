<div align="center">
  <picture>
    <img alt="LimeDB logo" src="logo/LimeDB_Logo-horizontal.png" height="100">
  </picture>
</div>
<br>

## LimeDB

**LimeDB** is a **distributed key-value database** built with **Java 21** and **Spring Boot**.  
It features a **coordinator-shard architecture** with **hash-based routing** for horizontal scalability.

LimeDB currently provides a **Redis-like API** with **PostgreSQL persistence** per shard as a starting point, with plans to evolve into a fully custom storage engine. This makes it simple to deploy and scale while learning distributed systems fundamentals.

---



## 🛣️ Roadmap

### ✅ Phase 1 (Completed)
- [x] **Hash-based Routing** - Same key always goes to same shard 
- [x] **PostgreSQL Persistence** - Durable storage per shard for a start
- [x] **Redis-like API** - GET, SET, DELETE operations  
- [x] **Coordinator Proxy** - Routes requests to appropriate shards
- [x] **Independent Shard Servers** - Scale by adding more shards  

### 🚧 Phase 2: Better Distribution
- [ ] **Consistent Hashing**: Replace modulo with a proper hash ring
- [ ] **Health Checks**: Automatic failover when shards go down
- [ ] **Dynamic Shard Addition/Removal**: Scale shards up and down
- [ ] **Key Migration & Rebalancing**: Move data when topology changes
- [ ] **Replication**: Primary-replica setup for high availability
- [ ] **Metrics**: Monitoring and observability

### 🔮 Phase 3: Custom Storage Engine
- [ ] **LSM Trees**: Replace PostgreSQL with custom key-value storage
- [ ] **Memory-Mapped Files**: Direct file system control
- [ ] **Custom Serialization**: Optimized data formats
- [ ] **WAL Implementation**: Write-ahead logging from scratch

### ⚡ Phase 4: Advanced Features
- [ ] **Custom Binary Protocol**: Move beyond HTTP/REST
- [ ] **Compression**: Custom compression algorithms
- [ ] **Cache Layers**: Multi-level caching strategies
- [ ] **Transaction Support**: ACID across multiple shards

---

## 🏗️ Architecture

```
                    +----------------+
                    |   Client App   |
                    +-------+--------+
                            |
                            | HTTP REST API
                            |
                    +-------v--------+
                    |  Coordinator   |
                    |  (Port 8080)   |
                    | Hash Routing   |
                    +-------+--------+
                            |
        +-----------+-------+-------+-----------+
        |                   |                   |
        | HTTP              | HTTP              | HTTP
        |                   |                   |
+-------v--------+ +-------v--------+ +-------v--------+
|    Shard 1     | |    Shard 2     | |    Shard 3     |
|  (Port 7001)   | |  (Port 7002)   | |  (Port 7003)   |
| PostgreSQL DB  | | PostgreSQL DB  | | PostgreSQL DB  |
|limedb_shard_1  | |limedb_shard_2  | |limedb_shard_3  |
+----------------+ +----------------+ +----------------+
```

**Routing Logic:**  
`hash(key) % num_shards = shard_index`

---

## 📦 Project Structure

```
limedb/
├── app/src/main/java/org/limedb/
│   ├── coordinator/
│   │   ├── config/           # RestTemplate configuration
│   │   └── core/
│   │       ├── controller/   # Coordinator REST API
│   │       └── service/      # Routing & Shard Registry
│   └── shard/
│       ├── config/           # Database configuration
│       └── core/
│           ├── controller/   # Shard REST API
│           ├── model/        # Entry (key-value) entity
│           ├── repository/   # JPA repositories
│           └── service/      # Shard business logic
├── scripts/                  # Python test scripts
└── README.md
```

---

## 🚀 Quick Start

### Prerequisites
- **Java 21**
- **PostgreSQL 14+** running on localhost:5432
- **Databases created**: `limedb_shard_1`, `limedb_shard_2`, `limedb_shard_3`

### 1. Start Shard Servers

```bash
# Terminal 1: Shard 1
./gradlew bootRun --args='--node.type=shard --server.port=7001 --shard.id=1'

# Terminal 2: Shard 2  
./gradlew bootRun --args='--node.type=shard --server.port=7002 --shard.id=2'

# Terminal 3: Shard 3
./gradlew bootRun --args='--node.type=shard --server.port=7003 --shard.id=3'
```

### 2. Start Coordinator

```bash
# Terminal 4: Coordinator
./gradlew bootRun --args='--node.type=coordinator --server.port=8080'
```

### 3. Test the API

```bash
# SET a key-value pair (routes to appropriate shard)
curl -X POST http://localhost:8080/api/v1/set \
  -H "Content-Type: application/json" \
  -d '{"key": "user:123", "value": "Alice"}'

# GET the value (routes to same shard)
curl http://localhost:8080/api/v1/get/user:123

# DELETE the key
curl -X DELETE http://localhost:8080/api/v1/del/user:123

# Check coordinator health
curl http://localhost:8080/api/v1/health
```


---

## 📡 API Reference

### Coordinator API (Port 8080)

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| `POST` | `/api/v1/set` | Store key-value pair | `{"key": "user:1", "value": "Alice"}` |
| `GET` | `/api/v1/get/{key}` | Retrieve value by key | `/api/v1/get/user:1` |
| `DELETE` | `/api/v1/del/{key}` | Delete key | `/api/v1/del/user:1` |
| `GET` | `/api/v1/health` | Coordinator status | Shows shard count & URLs |

### Shard API (Ports 7001-7003)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/set` | Direct shard storage |
| `GET` | `/api/v1/get/{key}` | Direct shard retrieval |
| `DELETE` | `/api/v1/del/{key}` | Direct shard deletion |

**Note:** Normally you'd only use the Coordinator API. Direct shard access is for debugging.


---

## ⚙️ Configuration

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
```

### Runtime Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `--node.type` | Node type (coordinator/shard) | `--node.type=shard` |
| `--server.port` | HTTP port | `--server.port=7001` |
| `--shard.id` | Shard identifier | `--shard.id=1` |

### Database Setup

```sql
-- Create databases for each shard
CREATE DATABASE limedb_shard_1;
CREATE DATABASE limedb_shard_2;  
CREATE DATABASE limedb_shard_3;

-- Create user (optional)
CREATE USER limedb WITH PASSWORD 'limedb';
GRANT ALL PRIVILEGES ON DATABASE limedb_shard_1 TO limedb;
GRANT ALL PRIVILEGES ON DATABASE limedb_shard_2 TO limedb;
GRANT ALL PRIVILEGES ON DATABASE limedb_shard_3 TO limedb;
```


---


## 🎯 Design Principles

- **Simplicity First** - Start simple, evolve to complex
- **Horizontal Scaling** - Add shards to scale storage & throughput  
- **Predictable Routing** - Same key always goes to same shard
- **Operational Simplicity** - Easy to deploy and monitor
- **Storage Evolution** - Start with PostgreSQL, evolve to custom engines

---

## 🛠️ Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Language** | Java 21 | Modern JVM with performance improvements |
| **Framework** | Spring Boot 3.5.6 | Web framework & dependency injection |
| **Database** | PostgreSQL 14+ | Persistent storage per shard |
| **ORM** | JPA/Hibernate | Database mapping & operations |
| **Build** | Gradle | Build automation & dependency management |
| **Architecture** | Coordinator-Shard | Distributed system pattern |
| **Routing** | Hash + Modulo | Simple deterministic routing |
| **Communication** | HTTP REST | Inter-service communication |


---

## � Testing

### Manual Testing
```bash
# Test hash routing - same key goes to same shard
curl -X POST http://localhost:8080/api/v1/set -H "Content-Type: application/json" -d '{"key": "test1", "value": "shard_test"}'
curl http://localhost:8080/api/v1/get/test1  # Should return "shard_test"

# Test persistence - restart shards and data should remain
curl -X POST http://localhost:8080/api/v1/set -H "Content-Type: application/json" -d '{"key": "persist", "value": "data"}'
# Restart shard servers
curl http://localhost:8080/api/v1/get/persist  # Should still return "data"
```

### Health Monitoring
```bash
# Check coordinator health
curl http://localhost:8080/api/v1/health
# Returns: {"status":"healthy","type":"coordinator","shardCount":3,"shards":["http://localhost:7001","http://localhost:7002","http://localhost:7003"]}
```

---

## 🤝 Contributing

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** (`git commit -m 'Add amazing feature'`)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

---

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 💡 Inspiration

LimeDB draws inspiration from:
- **Redis** - Simple key-value API and operational ease
- **Cassandra** - Distributed architecture patterns
- **PostgreSQL** - Reliable ACID storage engine
- **Spring Boot** - Developer-friendly Java framework

---

## 🏆 Why LimeDB?

- **🚀 Fast to Deploy** - Single JAR, familiar Spring Boot setup
- **📈 Horizontally Scalable** - Add shards as you grow
- **💾 Durable** - PostgreSQL persistence with ACID guarantees
- **🔍 Predictable** - Hash-based routing, same key → same shard
- **🛠️ Developer Friendly** - REST API, familiar tools
- **🔧 Extensible** - Clean architecture for future enhancements

**Ready to scale your key-value storage?** ⭐ Star the repo and get started!
