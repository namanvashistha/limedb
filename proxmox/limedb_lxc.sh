#!/usr/bin/env bash

# ==============================================================================
# LimeDB Proxmox LXC Setup Script
# ==============================================================================
# Usage:
# Run the following command on your Proxmox VE host shell:
# bash -c "$(wget -qLO - https://raw.githubusercontent.com/namanvashistha/limedb/main/proxmox/limedb_lxc.sh)"
# ==============================================================================

# Colors
YW=$(echo "\033[33m")
BL=$(echo "\033[36m")
RD=$(echo "\033[01;31m")
BGN=$(echo "\033[4;92m")
GN=$(echo "\033[1;92m")
DGN=$(echo "\033[32m")
CL=$(echo "\033[m")
CM="${GN}✓${CL}"
CROSS="${RD}✗${CL}"

function msg_info() {
    local msg="$1"
    echo -ne " ${YW}${msg}..."
}

function msg_ok() {
    local msg="$1"
    echo -e "${CM} ${GN}${msg}${CL}"
}

function msg_error() {
    local msg="$1"
    echo -e "${CROSS} ${RD}${msg}${CL}"
}

APP="LimeDB"
CT_PASSWORD="password"
DISK_SIZE="2G"
RAM_SIZE="512"
CORES="1"
OS_TEMPLATE="local:vztmpl/debian-12-standard_12.2-1_amd64.tar.zst" # Adjust based on available templates
STORAGE="local-lvm"

echo -e "${GN}Welcome to the ${APP} LXC Setup Script${CL}"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
  msg_error "Please run as root"
  exit 1
fi

# Find available CT ID
msg_info "Finding available Container ID"
CT_ID=100
while pct status $CT_ID >/dev/null 2>&1; do
    CT_ID=$((CT_ID + 1))
    if [ $CT_ID -gt 999 ]; then
        msg_error "No available CT ID found (checked 100-999)"
        exit 1
    fi
done
msg_ok "Using Container ID: ${CT_ID}"

# 1. Create Container
msg_info "Creating LXC Container (ID: ${CT_ID})"
pct create $CT_ID $OS_TEMPLATE \
    --hostname limedb \
    --cores $CORES \
    --memory $RAM_SIZE \
    --swap 512 \
    --storage $STORAGE \
    --password $CT_PASSWORD \
    --net0 name=eth0,bridge=vmbr0,ip=dhcp \
    --features nesting=1 \
    --unprivileged 1 \
    --rootfs $STORAGE:${DISK_SIZE} >/dev/null 2>&1

if [ $? -eq 0 ]; then
    msg_ok "Container Created"
else
    msg_error "Failed to create container. Check ID availability or template."
    exit 1
fi

# 2. Start Container
msg_info "Starting Container"
pct start $CT_ID
sleep 5 # Wait for startup
msg_ok "Container Started"

# 3. Install Dependencies & LimeDB
msg_info "Installing LimeDB"
pct exec $CT_ID -- bash -c "apt-get update && apt-get install -y curl ca-certificates wget" >/dev/null 2>&1
pct exec $CT_ID -- bash -c "wget -qO /usr/local/bin/limedb https://github.com/namanvashistha/limedb/releases/download/v0.0.2/limedb-linux-amd64"
pct exec $CT_ID -- bash -c "chmod +x /usr/local/bin/limedb"

# 4. Setup Service
pct exec $CT_ID -- bash -c "cat <<EOF > /etc/systemd/system/limedb.service
[Unit]
Description=LimeDB Key-Value Store
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/limedb
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF"

pct exec $CT_ID -- systemctl enable --now limedb >/dev/null 2>&1
msg_ok "LimeDB Installed & Started"

# 5. Get IP
IP=$(pct exec $CT_ID -- ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')

echo -e "${INFO}${YW} Access LimeDB at:${CL}"
echo -e "${TAB}${GATEWAY}${BGN}http://${IP}:7001${CL}"
