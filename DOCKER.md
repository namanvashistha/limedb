# LimeDB Docker Usage Guide

## Docker Image Successfully Created! 🎉

The LimeDB Docker image has been built successfully with the following features:

### Image Details:
## Image Details

- **Base Image**: Amazon Corretto 21 Alpine (optimized runtime)
- **Java Runtime**: Amazon Corretto 21 (JDK for build, JRE Alpine for runtime)
- **Build Tool**: Gradle 8.5 (manually installed)
- **Final Image Size**: ~444MB (38.8% smaller than Ubuntu-based version)
- **Security**: Runs as non-root user `limedb`
- **Multi-stage Build**: Optimized for production with minimal runtime dependencies

## Usage

### 1. Basic Usage (with external PostgreSQL)
```bash
# Run a single LimeDB node
docker run -p 7001:7001 \
  -e SPRING_DATASOURCE_URL=jdbc:postgresql://host.docker.internal:5432/limedb_node_1 \
  -e SPRING_DATASOURCE_USERNAME=limedb \
  -e SPRING_DATASOURCE_PASSWORD=limedb \
  limedb
```

### 2. Custom Configuration
```bash
# Run with custom node settings
docker run -p 7002:7002 \
  -e SPRING_DATASOURCE_URL=jdbc:postgresql://your-db-host:5432/limedb_node_2 \
  -e SPRING_DATASOURCE_USERNAME=your-user \
  -e SPRING_DATASOURCE_PASSWORD=your-password \
  limedb --server.port=7002 --node.id=2
```

### 3. Health Check
The container includes built-in health checks:
```bash
# Check container health
docker ps

# Manual health check
curl http://localhost:7001/cluster/state
```

## Prerequisites

You need PostgreSQL running with the appropriate databases created:
```sql
CREATE DATABASE limedb_node_1;
CREATE USER limedb WITH PASSWORD 'limedb';
GRANT ALL PRIVILEGES ON DATABASE limedb_node_1 TO limedb;
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SPRING_DATASOURCE_URL` | PostgreSQL connection URL | Required |
| `SPRING_DATASOURCE_USERNAME` | Database username | Required |
| `SPRING_DATASOURCE_PASSWORD` | Database password | Required |
| `NODE_PEERS` | Comma-separated peer URLs | See application.properties |

## Build Information

The Docker image was built using:
- **Multi-stage build** for optimal size
- **Amazon Corretto 21** for reliable Java runtime
- **Manual Gradle installation** to avoid layer corruption issues
- **Security best practices** with non-root user

## Troubleshooting

### Common Issues:
1. **Database Connection Errors**: Ensure PostgreSQL is running and accessible
2. **Port Conflicts**: Use different ports if 7001 is already in use
3. **Health Check Failures**: Wait for application to fully start (30-60 seconds)

The Docker image is ready for deployment! 🚀