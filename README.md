# MeshDB

**MeshDB** is a **horizontally scalable distributed key-value database** built in **Java**.  
It is designed to be **simple, lightweight, and high-performance**, with a **sharded architecture** that allows seamless horizontal scaling.

MeshDB starts with a **Spring Boot-based prototype** and will evolve into a **high-performance distributed storage system** with a custom networking layer and pluggable storage engine.

---

## ✅ Project Status

| Feature Area | Status |
|---------------|--------|
| Distributed architecture (Coordinator + Shards) | ✅ In Progress |
| Horizontal scaling | ✅ MVP |
| Consistent hashing ring | ✅ MVP |
| Basic CRUD operations | ✅ MVP |
| Rebalancing | 🚧 Planned |
| Persistence (RocksDB/SST) | 🚧 Planned |
| Replication & failover | 🔜 Future |
| gRPC/Binary protocol | 🔜 Future |
| CLI + Admin UI | 🔜 Future |

---

## 🔥 Features (MVP)

✅ Distributed Key-Value storage  
✅ Sharding using Consistent Hashing  
✅ Add/remove shard nodes dynamically  
✅ HTTP REST API  
✅ Java Client (MeshDB SDK)  
✅ Simple cluster coordination  
✅ Independent shard servers  
✅ Clean architecture – easy to evolve  

---

## 🏗️ Architecture

               +-------------------------+
               |       Client App        |
               +------------+------------+
                            |
                            v
                   MeshDB Client SDK
                            |
                            v
               +-------------------------+
               |  Coordinator (Spring)   |
               |  - Routing              |
               |  - Consistent Hashing   |
               |  - Cluster metadata     |
               +------------+------------+
                            |
   -----------------------------------------------------
   |                         |                         |
   v                         v                         v

+————+           +————+           +————+
|  Shard 1   |           |  Shard 2   |           |  Shard 3   |
| (Spring)   |           | (Spring)   |           | (Spring)   |
| In-memory  |           | In-memory  |           | In-memory  |
+————+           +————+           +————+

---

## 📦 Modules Layout

meshdb/
├── coordinator/        <– Routes client requests
├── shard-server/       <– Actual storage shards
├── common/             <– Shared utilities & models
├── client-java/        <– Java SDK
└── docs/               <– Architecture notes

---

## 🚀 Quick Start

### 1. Start Shard Servers

```bash
java -jar shard-server-0.0.1.jar --server.port=7001 --shard.id=1
java -jar shard-server-0.0.1.jar --server.port=7002 --shard.id=2
java -jar shard-server-0.0.1.jar --server.port=7003 --shard.id=3


⸻

2. Start Coordinator

java -jar coordinator-0.0.1.jar \
  --server.port=9000 \
  --meshdb.shards=http://localhost:7001,http://localhost:7002,http://localhost:7003


⸻

3. Use MeshDB Client (Java)

MeshDBClient db = new MeshDBClient("http://localhost:9000");

db.put("user:1", "{ \"name\": \"Alice\" }");
String user = db.get("user:1");
System.out.println(user);


⸻

✅ API (REST)

PUT

POST /put
{
  "key": "user:1",
  "value": "Alice"
}

GET

GET /get?key=user:1

DELETE

DELETE /delete?key=user:1


⸻

🔧 Configuration

Property	Description
meshdb.shards	List of shard URLs for coordinator
shard.id	Unique ID for shard server
server.port	Port to run coordinator/shard


⸻

🧱 Roadmap

✅ Phase 1 (MVP - Current)
	•	Coordinator + shards
	•	Consistent hashing
	•	Basic GET/PUT/DELETE

🚧 Phase 2
	•	Dynamic rebalancing
	•	Key migration between shards
	•	Hash ring improvements

🔨 Phase 3
	•	Persistence layer (RocksDB plugin)
	•	Write-Ahead Log (WAL)
	•	Snapshot support

🔄 Phase 4
	•	Replication (async primary-replica)
	•	Self-healing shard recovery

⚡ Phase 5
	•	Netty RPC transport
	•	Binary protocol
	•	Go/Python clients

⸻

🧭 Design Principles

✔ Simple before complex
✔ Horizontal scale from day 1
✔ Storage engine pluggable
✔ Dev-friendly APIs
✔ Real distributed system foundation

⸻

🛠️ Tech Stack

Component	Technology
Language	Java 17+
Framework	Spring Boot (MVP)
Hashing	Consistent Hash Ring
Build Tool	Gradle Kotlin
Client SDK	Java
Persistence	(Planned) RocksDB
Networking	REST now → Netty later


⸻

🤝 Contributing

We welcome contributions!
Check out CONTRIBUTING.md and feel free to open PRs.

⸻

📜 License

MIT License

⸻

⭐ Inspiration

MeshDB is inspired by:
	•	Redis (simplicity)
	•	Cassandra (distribution)
	•	Kafka (clustering)
	•	TiKV (architecture)

⸻

Want to help build the next big distributed database?
Star the repo and follow the roadmap! 🚀
