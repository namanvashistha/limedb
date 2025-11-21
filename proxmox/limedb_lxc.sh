#!/usr/bin/env bash

# ==============================================================================
# LimeDB Proxmox LXC Setup Script
# ==============================================================================
# Copyright (c) 2025 namanvashistha
# Author: namanvashistha
# License: MIT
# Source: https://github.com/namanvashistha/limedb
# ==============================================================================

set -euo pipefail
shopt -s inherit_errexit nullglob

# Colors and formatting
YW=$(echo "\033[33m")
BL=$(echo "\033[36m")
RD=$(echo "\033[01;31m")
BGN=$(echo "\033[4;92m")
GN=$(echo "\033[1;92m")
DGN=$(echo "\033[32m")
CL=$(echo "\033[m")
CM="${GN}✓${CL}"
CROSS="${RD}✗${CL}"
INFO="${BL}ℹ${CL}"
CREATING="${BL}🚀${CL}"
TAB="   "
GATEWAY="${BGN}"

# Configuration
APP="LimeDB"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-512}"
var_disk="${var_disk:-2}"
var_os="${var_os:-debian}"
var_version="${var_version:-12}"
var_unprivileged="${var_unprivileged:-1}"
PASSWORD="password"

# Functions
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

function header_info() {
    clear
    cat <<"EOF"
 __    _                ____  ____
/ /   (_)___ ___  ___  / __ \/ __ )
/ /   / / __ `__ \/ _ \/ / / / __  |
/ /___/ / / / / / /  __/ /_/ / /_/ /
/_____/_/_/ /_/ /_/\___/_____/_____/

EOF
echo -e "${CREATING}${GN}${APP} LXC Container Creation${CL}"
}

# Validate environment
if [[ $EUID -ne 0 ]]; then
    msg_error "This script must be run as root"
    exit 1
fi

if ! command -v pct &> /dev/null; then
    msg_error "Proxmox VE not detected"
    exit 1
fi

header_info

# Display configuration
echo -e "${INFO}${YW} Using Default Settings on node $(hostname)${CL}"
echo -e "${TAB}${YW}🆔  Container ID:${CL} Will be auto-detected"
echo -e "${TAB}${YW}🖥️  Operating System:${CL} ${var_os} (${var_version})"
echo -e "${TAB}${YW}📦  Container Type:${CL} $([ "$var_unprivileged" == "1" ] && echo "Unprivileged" || echo "Privileged")"
echo -e "${TAB}${YW}💾  Disk Size:${CL} ${var_disk} GB"
echo -e "${TAB}${YW}🧠  CPU Cores:${CL} ${var_cpu}"
echo -e "${TAB}${YW}🛠️  RAM Size:${CL} ${var_ram} MiB"
echo -e "${TAB}${CREATING}${GN}Creating a ${APP} LXC using the above settings${CL}"
echo ""

# Find available container ID
msg_info "Finding available Container ID"
CTID=100
while pct status $CTID &>/dev/null; do
    CTID=$((CTID + 1))
    if [[ $CTID -gt 999 ]]; then
        msg_error "No available Container ID found (checked 100-999)"
        exit 1
    fi
done
msg_ok "Using Container ID: ${CTID}"

# Detect storage
msg_info "Detecting available storage"
TEMPLATE_STORAGE=""
CONTAINER_STORAGE=""

# Find template storage
for storage in $(pvesm status -content vztmpl 2>/dev/null | awk 'NR>1 && /active/ {print $1}' | head -1); do
    if [[ -n "$storage" ]]; then
        TEMPLATE_STORAGE="$storage"
        break
    fi
done

# Find container storage
for storage in $(pvesm status -content rootdir 2>/dev/null | awk 'NR>1 && /active/ {print $1}' | head -1); do
    if [[ -n "$storage" ]]; then
        CONTAINER_STORAGE="$storage"
        break
    fi
done

if [[ -z "$TEMPLATE_STORAGE" || -z "$CONTAINER_STORAGE" ]]; then
    msg_error "No suitable storage found"
    exit 1
fi

# Get storage info
TEMPLATE_FREE=$(pvesm status -storage "$TEMPLATE_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", $4/1024/1024}')
TEMPLATE_USED=$(pvesm status -storage "$CONTAINER_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", ($3-$4)/1024/1024}')
CONTAINER_FREE=$(pvesm status -storage "$CONTAINER_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", $4/1024/1024}')
CONTAINER_USED=$(pvesm status -storage "$CONTAINER_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", ($3-$4)/1024/1024}')

echo -e "${CM} ${GN}Storage ${TEMPLATE_STORAGE} (Free: ${TEMPLATE_FREE}  Used: ${TEMPLATE_USED}) [Template]${CL}"
echo -e "${CM} ${GN}Storage ${CONTAINER_STORAGE} (Free: ${CONTAINER_FREE}  Used: ${CONTAINER_USED}) [Container]${CL}"

# Find OS template
msg_info "Finding OS template"
OS_TEMPLATE=""
case "$var_os" in
    "debian")
        TEMPLATE_PATTERN="debian-${var_version}-standard"
        ;;
    "ubuntu")
        TEMPLATE_PATTERN="ubuntu-${var_version}"
        ;;
    *)
        TEMPLATE_PATTERN="debian-12-standard"
        ;;
esac

# Get the latest matching template
OS_TEMPLATE=$(pveam list $TEMPLATE_STORAGE 2>/dev/null | grep "$TEMPLATE_PATTERN" | tail -1 | awk '{print $1}')

if [[ -z "$OS_TEMPLATE" ]]; then
    msg_error "No suitable OS template found for $var_os $var_version"
    echo "Available templates:"
    pveam list $TEMPLATE_STORAGE 2>/dev/null | grep -E "(debian|ubuntu)" | head -5
    exit 1
fi

TEMPLATE_NAME=$(basename "$OS_TEMPLATE" .tar.zst)
echo -e "${CM} ${GN}Template ${TEMPLATE_NAME} [${TEMPLATE_STORAGE}]${CL}"

# Create container
msg_info "Creating LXC Container"
pct create $CTID $TEMPLATE_STORAGE:vztmpl/$OS_TEMPLATE \
    --hostname limedb \
    --cores $var_cpu \
    --memory $var_ram \
    --swap 512 \
    --storage $CONTAINER_STORAGE \
    --password $PASSWORD \
    --net0 name=eth0,bridge=vmbr0,ip=dhcp \
    --features nesting=1 \
    --unprivileged $var_unprivileged \
    --rootfs $CONTAINER_STORAGE:${var_disk} &>/dev/null

if [[ $? -eq 0 ]]; then
    echo -e "${CM} ${GN}LXC Container ${CTID} was successfully created.${CL}"
else
    msg_error "Failed to create container"
    exit 1
fi

# Start container
msg_info "Starting LXC Container"
pct start $CTID &>/dev/null
echo -e "${CM} ${GN}Started LXC Container${CL}"

# Wait for network
msg_info "Waiting for network"
for i in {1..30}; do
    if pct exec $CTID -- ping -c1 8.8.8.8 &>/dev/null; then
        break
    fi
    if [[ $i -lt 30 ]]; then
        echo -ne " ${INFO} No network in LXC yet (try $i/30) – waiting..."
        sleep 2
        echo -ne "\r"
    fi
done

if pct exec $CTID -- ping -c1 8.8.8.8 &>/dev/null; then
    echo -e "${CM} ${GN}Network in LXC is reachable (ping)${CL}"
else
    msg_error "Network not reachable after 60 seconds"
    exit 1
fi

# Get container IP
IP=$(pct exec $CTID -- ip route get 1 2>/dev/null | awk '{print $7}' | head -1)

# Install LimeDB in container
echo "Extracting templates from packages: 100%"
echo -e "${CM} ${GN}Customized LXC Container${CL}"

# Install dependencies and LimeDB
pct exec $CTID -- bash -c "
    apt-get update &>/dev/null
    apt-get install -y curl ca-certificates wget &>/dev/null
    
    # Get latest version
    LATEST_VERSION=\$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '\"tag_name\":' | sed -E 's/.*\"([^\"]+)\".*/\1/')
    
    if [ -z \"\$LATEST_VERSION\" ]; then
        LATEST_VERSION=\"v0.0.2\"
    fi
    
    # Download and install LimeDB
    wget -qO /usr/local/bin/limedb \"https://github.com/namanvashistha/limedb/releases/download/\${LATEST_VERSION}/limedb-linux-amd64\"
    chmod +x /usr/local/bin/limedb
    
    # Create systemd service
    cat > /etc/systemd/system/limedb.service << 'EOF'
[Unit]
Description=LimeDB Key-Value Store
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/limedb
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    
    # Enable and start service
    systemctl daemon-reload
    systemctl enable --now limedb.service &>/dev/null
    
    # Create MOTD
    cat > /etc/motd << 'EOF'
   __    _                ____  ____
  / /   (_)___ ___  ___  / __ \/ __ )
 / /   / / __ `__ \/ _ \/ / / / __  |
/ /___/ / / / / / /  __/ /_/ / /_/ /
/_____/_/_/ /_/ /_/\___/_____/_____/

LimeDB - Fast Key-Value Store
Access: http://$(ip route get 1 | awk '{print \$7}' | head -1):7001
Docs: https://github.com/namanvashistha/limedb

EOF
"

# Final output
echo ""
echo -e "${CM} ${GN}Completed Successfully!${CL}"
echo ""
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW} Access it using the following URL:${CL}"
echo -e "${TAB}${GATEWAY}${BGN}http://${IP}:7001${CL}"
echo ""
echo -e "${YW}Container ID:${CL} ${CTID}"
echo -e "${YW}Username:${CL} root"
echo -e "${YW}Password:${CL} ${PASSWORD}"
echo ""
echo -e "${DGN}Management commands:${CL}"
echo -e "${TAB}${YW}pct enter ${CTID}${CL} - Enter container"
echo -e "${TAB}${YW}pct stop ${CTID}${CL} - Stop container"
echo -e "${TAB}${YW}pct start ${CTID}${CL} - Start container"
