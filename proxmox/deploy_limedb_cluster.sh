#!/bin/bash

# LimeDB Multi-Node LXC Deployment Script
# Usage: ./deploy_limedb_cluster.sh [NUM_NODES]
# Example: ./deploy_limedb_cluster.sh 5

set -e

# Configuration
NUM_NODES="${NUM_NODES:-2}" # Default to 3 nodes if not specified
BASE_PORT=8484
SEED_NODE_IP=""
SEED_NODE_URL=""

echo "========================================="
echo "  LimeDB Multi-Node Cluster Deployment"
echo "========================================="
echo "Creating $NUM_NODES LimeDB nodes..."
echo ""

# Fetch latest release tag
TAG=$(wget -qO- https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep -oP '"tag_name": "\K[^"]+')
echo "✓ Latest LimeDB version: $TAG"
echo ""

# Download the base LXC script
SCRIPT_URL="https://raw.githubusercontent.com/namanvashistha/limedb/$TAG/proxmox/limedb_lxc.sh"
TMP_SCRIPT=$(mktemp)
wget -qO "$TMP_SCRIPT" "$SCRIPT_URL"
chmod +x "$TMP_SCRIPT"

echo "Creating nodes..."
echo ""

# Track created container IDs
CONTAINER_IDS=()

# Create each node
for i in $(seq 1 $NUM_NODES); do
    NODE_NUM=$i
    
    echo "-------------------------------------------"
    echo "Node $NODE_NUM/$NUM_NODES"
    
    if [ $i -eq 1 ]; then
        echo "  Role: SEED NODE"
        echo ""
        echo "  Creating seed node..."
        
        # Create seed node without peers
        bash "$TMP_SCRIPT"
        
        # Get the container ID of the seed node (last created container)
        SEED_CTID=$(pct list 2>/dev/null | awk 'NR>1 {print $1}' | sort -n | tail -1)
        CONTAINER_IDS+=($SEED_CTID)
        
        # Wait for container to be fully ready
        sleep 5
        
        # Get the IP of the seed container
        SEED_NODE_IP=$(pct exec $SEED_CTID -- hostname -I 2>/dev/null | awk '{print $1}')
        
        if [[ -z "$SEED_NODE_IP" ]]; then
            # Fallback: try to get from network config
            SEED_NODE_IP=$(pct config $SEED_CTID | grep 'net0:' | grep -oP 'ip=\K[0-9.]+' | head -1)
        fi
        
        if [[ -z "$SEED_NODE_IP" ]]; then
            echo "ERROR: Could not determine seed node IP"
            exit 1
        fi
        
        SEED_NODE_URL="http://$SEED_NODE_IP:$BASE_PORT"
        
        echo "✓ Seed node created successfully"
        echo "  Container ID: $SEED_CTID"
        echo "  IP Address: $SEED_NODE_IP"
        echo "  Node URL: $SEED_NODE_URL"
        echo ""
    else
        echo "  Role: PEER NODE"
        echo "  Seed: $SEED_NODE_URL"
        echo ""
        echo "  Creating peer node..."
        
        # Create peer node connected to seed
        bash "$TMP_SCRIPT" --peers "$SEED_NODE_URL"
        
        # Get the container ID of this node
        PEER_CTID=$(pct list 2>/dev/null | awk 'NR>1 {print $1}' | sort -n | tail -1)
        CONTAINER_IDS+=($PEER_CTID)
        
        echo "✓ Node $NODE_NUM created successfully"
        echo "  Container ID: $PEER_CTID"
        echo ""
    fi
    
    # Small delay between node creations
    if [ $i -lt $NUM_NODES ]; then
        sleep 2
    fi
done

# Cleanup
rm -f "$TMP_SCRIPT"

echo "========================================="
echo "  Deployment Complete!"
echo "========================================="
echo ""
echo "Cluster Summary:"
echo "  Total Nodes: $NUM_NODES"
echo "  Seed Node: Container ${CONTAINER_IDS[0]} @ $SEED_NODE_URL"
echo ""
echo "All Nodes:"
for i in "${!CONTAINER_IDS[@]}"; do
    CTID=${CONTAINER_IDS[$i]}
    NODE_IP=$(pct exec $CTID -- hostname -I 2>/dev/null | awk '{print $1}' || echo "pending")
    
    if [ $i -eq 0 ]; then
        echo "  Node $((i+1)) (SEED):  Container $CTID - http://$NODE_IP:$BASE_PORT"
    else
        echo "  Node $((i+1)):         Container $CTID - http://$NODE_IP:$BASE_PORT"
    fi
done
echo ""
echo "Access Management:"
echo "  View cluster:     curl http://$SEED_NODE_IP:$BASE_PORT/api/v1/gossip"
echo "  Enter any node:   pct enter <CONTAINER_ID>"
echo "  View logs:        pct exec <CONTAINER_ID> journalctl -u limedb -f"
echo ""
echo "========================================="
