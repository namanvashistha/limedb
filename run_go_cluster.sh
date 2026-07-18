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

echo -e "${GREEN}Starting ${NUM_NODES} nodes...${NC}"

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

# Start Nodes. Every node gets the FULL initial member list as peers so the
# whole initial cluster auto-joins the routing ring with an identical view.
# Nodes added later (not in this list) are discovered via gossip and go
# through the manual admin activate/bootstrap flow.
for ((i=0; i<NUM_NODES; i++)); do
    PORT=$((START_PORT + i))
    NODE_URL="http://localhost:${PORT}"

    PEERS=""
    for ((j=0; j<NUM_NODES; j++)); do
        if [ $j -ne $i ]; then
            PEER="http://localhost:$((START_PORT + j))"
            if [ -z "$PEERS" ]; then
                PEERS="$PEER"
            else
                PEERS="$PEERS,$PEER"
            fi
        fi
    done

    echo "Starting Node at ${NODE_URL} with peers ${PEERS}..."
    $BINARY_PATH -server.port $PORT -node.url "$NODE_URL" -node.peers "$PEERS" -node.routing.virtual-nodes 1 -otel.endpoint "" &
    PIDS+=($!)
done

echo -e "${GREEN}Cluster is running! Press Ctrl+C to stop.${NC}"
echo -e "Try: curl http://localhost:${START_PORT}/api/v1/cluster/state"

# Wait for all background processes
wait
