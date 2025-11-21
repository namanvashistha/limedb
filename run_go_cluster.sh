#!/bin/bash

# Configuration
START_PORT=7001
NUM_NODES=50
BINARY_PATH="./go-node/limedb"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building LimeDB Go Node...${NC}"
cd go-node
go build -o limedb cmd/server/main.go
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi
cd ..

# Generate Peers List
PEERS=""
for ((i=0; i<NUM_NODES; i++)); do
    PORT=$((START_PORT + i))
    if [ $i -gt 0 ]; then
        PEERS="${PEERS},"
    fi
    PEERS="${PEERS}http://localhost:${PORT}"
done

echo -e "${GREEN}Starting ${NUM_NODES} nodes...${NC}"
echo -e "Peers: ${PEERS}"

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
    NODE_ID=$((i + 1))
    PORT=$((START_PORT + i))
    
    echo "Starting Node ${NODE_ID} on port ${PORT}..."
    $BINARY_PATH -node.id $NODE_ID -server.port $PORT -node.peers "$PEERS" &
    PIDS+=($!)
done

echo -e "${GREEN}Cluster is running! Press Ctrl+C to stop.${NC}"
echo -e "Try: curl http://localhost:${START_PORT}/api/v1/cluster/state"

# Wait for all background processes
wait
