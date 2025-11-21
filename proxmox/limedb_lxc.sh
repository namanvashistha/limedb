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
YW='\033[33m'     # Yellow
BL='\033[36m'     # Blue  
RD='\033[01;31m'  # Red
BGN='\033[4;92m'  # Background Green
GN='\033[1;92m'   # Green
DGN='\033[32m'    # Dark Green
CL='\033[m'       # Clear
CM="${GN}✓${CL}"
CROSS="${RD}✗${CL}"
INFO="${BL}ℹ${CL}"
CREATING="${BL}🚀${CL}"
SUCCESS="${GN}🎉${CL}"
WARNING="${YW}⚠${CL}"
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
  ╭─────────────────────────────────────────────────────╮
  │                                                     │
  │    __    _                ____  ____                │
  │   / /   (_)___ ___  ___  / __ \/ __ )               │
  │  / /   / / __ `__ \/ _ \/ / / / __  |               │
  │ / /___/ / / / / / /  __/ /_/ / /_/ /                │
  │/_____/_/_/ /_/ /_/\___/_____/_____/                 │
  │                                                     │
  │        Fast Key-Value Store for Modern Apps        │
  │                                                     │
  ╰─────────────────────────────────────────────────────╯

EOF
echo -e "${CREATING} ${GN}${APP} LXC Container Setup${CL}"
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"
echo ""
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
echo -e "🔧 ${YW}Configuration${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${YW}Node:${CL}           $(hostname)"
echo -e "│ ${YW}Container ID:${CL}   Will be auto-detected"
echo -e "│ ${YW}OS:${CL}             ${var_os} ${var_version}"
echo -e "│ ${YW}Type:${CL}           $([ "$var_unprivileged" == "1" ] && echo "Unprivileged" || echo "Privileged")"
echo -e "│ ${YW}Storage:${CL}        ${var_disk} GB"
echo -e "│ ${YW}CPU Cores:${CL}      ${var_cpu}"
echo -e "│ ${YW}Memory:${CL}         ${var_ram} MiB"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""
echo -e "${CREATING} ${GN}Initializing ${APP} container deployment...${CL}"
echo ""

# Find available container ID (optimized)
msg_info "Finding available Container ID"
# Get all existing container IDs at once
EXISTING_IDS=$(pct list 2>/dev/null | awk 'NR>1 {print $1}' | sort -n)
CTID=100
for existing_id in $EXISTING_IDS; do
    if [[ $CTID -eq $existing_id ]]; then
        CTID=$((CTID + 1))
    elif [[ $CTID -lt $existing_id ]]; then
        break
    fi
done

if [[ $CTID -gt 999 ]]; then
    msg_error "No available Container ID found (checked 100-999)"
    exit 1
fi
msg_ok "Using Container ID: ${CTID}"

# Detect storage (robust)
msg_info "Detecting available storage"

# Get all storage info and find template storage
TEMPLATE_STORAGE=""
CONTAINER_STORAGE=""

# Find template storage (supports vztmpl content)
for storage in $(pvesm status -content vztmpl 2>/dev/null | awk 'NR>1 && /active/ {print $1}'); do
    if [[ -n "$storage" ]]; then
        TEMPLATE_STORAGE="$storage"
        break
    fi
done

# Find container storage (supports rootdir content)
for storage in $(pvesm status -content rootdir 2>/dev/null | awk 'NR>1 && /active/ {print $1}'); do
    if [[ -n "$storage" ]]; then
        CONTAINER_STORAGE="$storage"
        break
    fi
done

# Fallback: use any available storage if specific content types not found
if [[ -z "$TEMPLATE_STORAGE" ]]; then
    TEMPLATE_STORAGE=$(pvesm status 2>/dev/null | awk 'NR>1 && /active/ && /local/ {print $1}' | head -1)
fi

if [[ -z "$CONTAINER_STORAGE" ]]; then
    CONTAINER_STORAGE=$(pvesm status 2>/dev/null | awk 'NR>1 && /active/ && /local-lvm/ {print $1}' | head -1)
    if [[ -z "$CONTAINER_STORAGE" ]]; then
        CONTAINER_STORAGE=$(pvesm status 2>/dev/null | awk 'NR>1 && /active/ {print $1}' | head -1)
    fi
fi

if [[ -z "$TEMPLATE_STORAGE" || -z "$CONTAINER_STORAGE" ]]; then
    msg_error "No suitable storage found"
    echo "Available storage:"
    pvesm status 2>/dev/null | awk 'NR>1 {print "  " $1 " (" $2 ")"}'
    exit 1
fi

# Get storage info
TEMPLATE_FREE=$(pvesm status -storage "$TEMPLATE_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", $4/1024/1024}')
CONTAINER_FREE=$(pvesm status -storage "$CONTAINER_STORAGE" 2>/dev/null | awk 'NR>1 {printf "%.1fGB", $4/1024/1024}')

echo -e "${CM} ${GN}Template Storage${CL}   ${TEMPLATE_STORAGE} ${DGN}(${TEMPLATE_FREE} available)${CL}"
echo -e "${CM} ${GN}Container Storage${CL}  ${CONTAINER_STORAGE} ${DGN}(${CONTAINER_FREE} available)${CL}"

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

# Get the latest matching template (just the filename)
OS_TEMPLATE=$(pveam list $TEMPLATE_STORAGE 2>/dev/null | grep "$TEMPLATE_PATTERN" | tail -1 | awk '{print $1}')

if [[ -z "$OS_TEMPLATE" ]]; then
    msg_error "No suitable OS template found for $var_os $var_version"
    echo "Available templates:"
    pveam list $TEMPLATE_STORAGE 2>/dev/null | grep -E "(debian|ubuntu)" | head -5
    exit 1
fi

TEMPLATE_NAME=$(basename "$OS_TEMPLATE" .tar.zst)
echo -e "${CM} ${GN}OS Template${CL}        ${TEMPLATE_NAME}"

# Create container
msg_info "Creating LXC Container"

# Build the pct create command with proper template path
# The OS_TEMPLATE already contains the full filename, just need storage:vztmpl/filename
TEMPLATE_PATH="${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE_NAME}.tar.zst"

# Show container specifications being created
echo -ne "\r ${DGN}Creating container ${CTID} with ${var_cpu} core(s), ${var_ram}MB RAM, ${var_disk}GB storage...${CL}"

# Generate static IP based on CTID
HOST_IP=$(ip route get 1 2>/dev/null | awk '{print $7}' | head -1)
if [[ -n "$HOST_IP" ]]; then
    # Extract network portion (first 3 octets) and use CTID as last octet
    NETWORK_PREFIX=$(echo "$HOST_IP" | cut -d. -f1-3)
    STATIC_IP="${NETWORK_PREFIX}.${CTID}/24"
    GATEWAY_IP="${NETWORK_PREFIX}.1"
    NET_CONFIG="ip=${STATIC_IP},gw=${GATEWAY_IP}"
else
    # Fallback to DHCP if we can't determine host IP
    NET_CONFIG="dhcp"
fi

echo -ne "\r ${DGN}Assigning static IP: ${NETWORK_PREFIX}.${CTID}${CL}"

# Create container in background with progress indication
(
    pct create $CTID "$TEMPLATE_PATH" \
        --hostname limedb \
        --cores $var_cpu \
        --memory $var_ram \
        --swap 512 \
        --storage $CONTAINER_STORAGE \
        --password $PASSWORD \
        --net0 name=eth0,bridge=vmbr0,${NET_CONFIG} \
        --features nesting=1 \
        --unprivileged $var_unprivileged \
        --rootfs $CONTAINER_STORAGE:${var_disk} > /tmp/pct_output.log 2>/tmp/pct_error.log
    echo $? > /tmp/pct_result
) &

PCT_PID=$!

# Show progress while waiting
DOTS=""
COUNTER=0
while kill -0 $PCT_PID 2>/dev/null; do
    DOTS="${DOTS}."
    if [[ ${#DOTS} -gt 3 ]]; then
        DOTS=""
    fi
    echo -ne "\r ${YW}Creating LXC Container${DOTS}${CL}"
    sleep 1
    COUNTER=$((COUNTER + 1))
    
    # Timeout after 120 seconds
    if [[ $COUNTER -gt 120 ]]; then
        kill $PCT_PID 2>/dev/null
        echo -ne "\r"
        msg_error "Container creation timed out after 2 minutes"
        echo "Partial output:"
        cat /tmp/pct_output.log 2>/dev/null || echo "No output available"
        cat /tmp/pct_error.log 2>/dev/null
        exit 1
    fi
done

# Get the result
wait $PCT_PID
CREATE_RESULT=$(cat /tmp/pct_result 2>/dev/null || echo 1)

echo -ne "\r"
if [[ $CREATE_RESULT -eq 0 ]]; then
    echo -e "${CM} ${GN}Container created successfully${CL} ${DGN}(ID: ${CTID})${CL}"
else
    msg_error "Container creation failed"
    echo -e "${WARNING} ${YW}Troubleshooting information:${CL}"
    echo "Command output:"
    cat /tmp/pct_output.log 2>/dev/null || echo "No output available"
    echo -e "\n${RD}Error details:${CL}"
    cat /tmp/pct_error.log 2>/dev/null || echo "No error details available"
    exit 1
fi

# Cleanup temp files
rm -f /tmp/pct_output.log /tmp/pct_error.log /tmp/pct_result

# Start container
msg_info "Starting container"
if pct start $CTID 2>/dev/null; then
    echo -e "${CM} ${GN}Container started${CL}"
else
    msg_error "Failed to start container"
    echo -e "${WARNING} ${YW}Try manually: pct start ${CTID}${CL}"
    exit 1
fi

# Wait for container to be ready and network available (optimized)
msg_info "Waiting for network connectivity"
for i in {1..15}; do
    if pct exec $CTID -- test -f /bin/bash 2>/dev/null; then
        # Container is responsive, test network with lighter check
        if pct exec $CTID -- timeout 2 ping -c1 8.8.8.8 &>/dev/null; then
            echo -e "${CM} ${GN}Network ready${CL}"
            break
        fi
    fi
    echo -ne "\r ${YW}Waiting for network connectivity... (${i}/15)${CL}"
    sleep 2
    if [[ $i -eq 15 ]]; then
        echo -ne "\r"
        msg_error "Network not ready after 30 seconds"
        echo -e "${WARNING} ${YW}Container may still be starting. Try: pct enter ${CTID}${CL}"
        exit 1
    fi
done

# Get container IP (use static IP if configured)
if [[ -n "$NETWORK_PREFIX" ]]; then
    IP="${NETWORK_PREFIX}.${CTID}"
else
    IP=$(pct exec $CTID -- ip route get 1 2>/dev/null | awk '{print $7}' | head -1)
fi

# Install LimeDB in container
echo ""
echo -e "📦 ${YW}Installing LimeDB...${CL}"
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"

# Note: LimeDB now requires mandatory node URL specification
# The service is configured as a single-node cluster by default
# For multi-node clusters, edit the systemd service file after installation

# Install dependencies and LimeDB (optimized single operation)
echo -ne " ${YW}Installing LimeDB (all steps)...${CL}"
pct exec $CTID -- bash -c "
    # Update and install dependencies
    apt-get update &>/dev/null
    apt-get install -y curl ca-certificates wget &>/dev/null
    
    # Get latest version and download in parallel
    LATEST_VERSION=\$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '\"tag_name\":' | sed -E 's/.*\"([^\"]+)\".*/\1/')
    
    if [ -z \"\$LATEST_VERSION\" ]; then
        LATEST_VERSION=\"v0.0.2\"
    fi
    
    # Download and install LimeDB
    wget -qO /usr/local/bin/limedb \"https://github.com/namanvashistha/limedb/releases/download/\${LATEST_VERSION}/limedb-linux-amd64\" &
    
    # Get container IP for node URL
    CONTAINER_IP=\$(ip route get 1 2>/dev/null | awk '{print \$7}' | head -1)
    if [ -z \"\$CONTAINER_IP\" ]; then
        CONTAINER_IP=\"localhost\"
    fi
    
    # Create systemd service while download happens
    cat > /etc/systemd/system/limedb.service << EOF
[Unit]
Description=LimeDB Key-Value Store
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/limedb -server.port 8484 -node.url \"http://\${CONTAINER_IP}:8484\" -node.peers \"http://\${CONTAINER_IP}:8484\"
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    
    # Create MOTD while download happens
    cat > /etc/motd << 'MOTDEOF'
  ╭─────────────────────────────────────────────────────╮
  │                                                     │
  │    __    _                ____  ____                │
  │   / /   (_)___ ___  ___  / __ \/ __ )               │
  │  / /   / / __ \`__ \/ _ \/ / / / __  |               │
  │ / /___/ / / / / / /  __/ /_/ / /_/ /                │
  │/_____/_/_/ /_/ /_/\___/_____/_____/                 │
  │                                                     │
  │        Fast Key-Value Store for Modern Apps        │
  │                                                     │
  ╰─────────────────────────────────────────────────────╯

  🌐 Access: http://\$(ip route get 1 | awk '{print \$7}' | head -1):8484
  📚 Documentation: https://github.com/namanvashistha/limedb
  🔧 Management: systemctl [start|stop|restart] limedb
  📋 Node URL: http://\$(ip route get 1 | awk '{print \$7}' | head -1):8484

MOTDEOF
    
    # Enable auto-login for root user on console
    mkdir -p /etc/systemd/system/getty@tty1.service.d
    cat > /etc/systemd/system/getty@tty1.service.d/autologin.conf << 'AUTOEOF'
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin root --noclear %I \$TERM
AUTOEOF
    
    # Enable auto-login for container console
    mkdir -p /etc/systemd/system/container-getty@1.service.d
    cat > /etc/systemd/system/container-getty@1.service.d/autologin.conf << 'AUTOEOF2'
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin root --noclear --keep-baud console 115200,38400,9600 \$TERM
AUTOEOF2
    
    # Reload systemd to apply auto-login
    systemctl daemon-reload
    
    # Wait for download to complete
    wait
    chmod +x /usr/local/bin/limedb
    
    # Enable and start service
    systemctl daemon-reload
    systemctl enable --now limedb.service &>/dev/null
    
    echo \"\$LATEST_VERSION\"
"
INSTALLED_VERSION=\$?
echo -e "\r${CM} ${GN}LimeDB installation completed${CL}"

# Final output
echo ""
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"
echo -e "${SUCCESS} ${GN}LimeDB installation completed successfully!${CL}"
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"
echo ""

# Container Information Box
echo -e "📋 ${YW}Container Information${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${YW}Container ID:${CL}   ${CTID}"
echo -e "│ ${YW}IP Address:${CL}     ${IP}$([ -n "$NETWORK_PREFIX" ] && echo " (static)" || echo " (dhcp)")"
echo -e "│ ${YW}Username:${CL}       root"
echo -e "│ ${YW}Password:${CL}       ${PASSWORD}"
echo -e "│ ${YW}Service Port:${CL}   8484"
if [[ -n "$NETWORK_PREFIX" ]]; then
echo -e "│ ${YW}Gateway:${CL}        ${GATEWAY_IP}"
fi
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

# Access Information
echo -e "🌐 ${YW}Access Information${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${YW}Web Interface:${CL}  ${GATEWAY}http://${IP}:8484${CL}"
echo -e "│ ${YW}API Endpoint:${CL}   ${GATEWAY}http://${IP}:8484${CL}"
echo -e "│ ${YW}Health Check:${CL}   ${GATEWAY}curl http://${IP}:8484${CL}"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

# Management Commands
echo -e "🔧 ${YW}Management Commands${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${DGN}Enter container:${CL}       pct enter ${CTID}"
echo -e "│ ${DGN}Stop container:${CL}        pct stop ${CTID}"
echo -e "│ ${DGN}Start container:${CL}       pct start ${CTID}"
echo -e "│ ${DGN}Restart LimeDB:${CL}        pct exec ${CTID} systemctl restart limedb"
echo -e "│ ${DGN}View logs:${CL}             pct exec ${CTID} journalctl -u limedb -f"
echo -e "│ ${DGN}Check status:${CL}          pct exec ${CTID} systemctl status limedb"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

# Cluster Configuration
echo -e "🔗 ${YW}Cluster Configuration${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${YW}Single Node:${CL}           Currently configured"
echo -e "│ ${YW}Multi-Node Setup:${CL}      Edit /etc/systemd/system/limedb.service"
echo -e "│ ${YW}Node URL:${CL}              -node.url \"http://${IP}:8484\""
echo -e "│ ${YW}Peers Example:${CL}         -node.peers \"http://ip1:8484,http://ip2:8484\""
echo -e "│ ${DGN}Reload after edit:${CL}     systemctl daemon-reload && systemctl restart limedb"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

echo -e "${SUCCESS} ${GN}Enjoy using LimeDB!${CL} ${DGN}Visit: https://github.com/namanvashistha/limedb${CL}"
echo ""
