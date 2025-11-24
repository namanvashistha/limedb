#!/bin/bash

# Configuration
START_PORT=7001
NUM_NODES=${NUM_NODES:-5}
BINARY_PATH="./build/limedb"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building LimeDB Go Node...${NC}"
go build -o build/limedb ./cmd/server/main.go
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

# Set the first node as the seed peer for all others
SEED_PEER="http://localhost:${START_PORT}"

echo -e "${GREEN}Starting ${NUM_NODES} nodes...${NC}"
echo -e "Seed peer: ${SEED_PEER}"

# Array to keep track of PIDs
PIDS=()

# Function to kill all nodes on script exit
cleanup() {
    echo -e "\n${RED}Stopping all nodes...${NC}"
    for pid in "${PIDS[@]}"; do
        kill $pid 2>/dev/null
    done
    wait
    echo -e "${GREEN}Cluster stopped.${NC}"
}

# Trap SIGINT (Ctrl+C) and SIGTERM
trap cleanup SIGINT SIGTERM

# Start Nodes
for ((i=0; i<NUM_NODES; i++)); do
    PORT=$((START_PORT + i))
    NODE_URL="http://localhost:${PORT}"
    
    if [ $i -eq 0 ]; then
        # First node starts without peers (seed node)
        echo "Starting Seed Node at ${NODE_URL}..."
        $BINARY_PATH -server.port $PORT -node.url "$NODE_URL" -node.routing.virtual-nodes 1 &
    else
        # Each node connects to the previous node
        PREV_PORT=$((START_PORT + i - 1))
        PREV_PEER="http://localhost:${PREV_PORT}"
        echo "Starting Node at ${NODE_URL} with peer ${PREV_PEER}..."
        $BINARY_PATH -server.port $PORT -node.url "$NODE_URL" -node.peers "$PREV_PEER"  -node.routing.virtual-nodes 1 &
    fi
    PIDS+=($!)
done

echo -e "${GREEN}Cluster is running! Press Ctrl+C to stop.${NC}"
echo -e "Try: curl http://localhost:${START_PORT}/api/v1/cluster/state"

# Wait for all background processes
wait
