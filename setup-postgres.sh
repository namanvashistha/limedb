#!/bin/bash

# Setup script for LimeDB PostgreSQL databases
# This script creates the required databases and user for LimeDB shards

echo "Setting up PostgreSQL databases for LimeDB shards..."

# Connect to PostgreSQL and create databases (using default user, not postgres)
psql -d postgres -c "CREATE USER limedb WITH PASSWORD 'limedb';" 2>/dev/null || echo "User limedb already exists or cannot create"

# Create databases for shards 1, 2, and 3
for i in {1..3}; do
    DB_NAME="limedb_shard_$i"
    echo "Creating database: $DB_NAME"
    psql -d postgres -c "CREATE DATABASE $DB_NAME;" 2>/dev/null || echo "Database $DB_NAME already exists"
    psql -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO limedb;" 2>/dev/null
done

echo "PostgreSQL setup complete!"
echo ""
echo "You can now run your shards with:"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7001 --shard.id=1'"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7002 --shard.id=2'"
echo "  ./gradlew bootRun --args='--node.type=shard --server.port=7003 --shard.id=3'"