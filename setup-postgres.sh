#!/bin/bash

# Setup script for LimeDB PostgreSQL databases
# This script creates the required databases and user for LimeDB nodes

echo "Setting up PostgreSQL databases for LimeDB nodes..."

# Connect to PostgreSQL and create databases (using default user, not postgres)
psql -d postgres -c "CREATE USER limedb WITH PASSWORD 'limedb';" 2>/dev/null || echo "User limedb already exists or cannot create"

# Create databases for nodes 1, 2, 3, 4, and 5
for i in {1..5}; do
    DB_NAME="limedb_node_$i"
    echo "Creating database: $DB_NAME"
    psql -d postgres -c "CREATE DATABASE $DB_NAME;" 2>/dev/null || echo "Database $DB_NAME already exists"
    psql -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO limedb;" 2>/dev/null
done

echo "PostgreSQL setup complete!"
echo ""
echo "You can now run your nodes with:"
echo "  ./gradlew bootRun --args='--node.type=node --server.port=7001 --node.id=1'"
echo "  ./gradlew bootRun --args='--node.type=node --server.port=7002 --node.id=2'"
echo "  ./gradlew bootRun --args='--node.type=node --server.port=7003 --node.id=3'"