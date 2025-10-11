#!/bin/bash

# Setup script for MeshDB PostgreSQL databases
# This script creates the required databases and user for MeshDB shards

echo "Setting up PostgreSQL databases for MeshDB shards..."

# Connect to PostgreSQL and create databases (using default user, not postgres)
psql -d postgres -c "CREATE USER meshdb WITH PASSWORD 'meshdb';" 2>/dev/null || echo "User meshdb already exists or cannot create"

# Create databases for shards 1, 2, and 3
for i in {1..3}; do
    DB_NAME="meshdb_shard_$i"
    echo "Creating database: $DB_NAME"
    psql -d postgres -c "CREATE DATABASE $DB_NAME;" 2>/dev/null || echo "Database $DB_NAME already exists"
    psql -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO meshdb;" 2>/dev/null
done

echo "PostgreSQL setup complete!"
echo ""
echo "You can now run your shards with:"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7001 --shard.id=1'"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7002 --shard.id=2'"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7003 --shard.id=3'"