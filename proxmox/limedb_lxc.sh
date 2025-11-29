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
var_ram="${var_ram:-150}"
var_disk="${var_disk:-2}"
var_os="${var_os:-alpine}"
var_version="${var_version:-3.22}"
var_unprivileged="${var_unprivileged:-1}"
PASSWORD="password"

# Grafana Cloud Configuration - Read from ~/.grafana/ if available
GRAFANA_CONFIG_DIR="$HOME/.grafana"
if [[ -f "$GRAFANA_CONFIG_DIR/config" ]]; then
    source "$GRAFANA_CONFIG_DIR/config"
fi

# Environment variable fallbacks
GRAFANA_OTLP_ENDPOINT="${GRAFANA_OTLP_ENDPOINT:-}"
GRAFANA_CLOUD_USERNAME="${GRAFANA_CLOUD_USERNAME:-}"
GRAFANA_CLOUD_PASSWORD="${GRAFANA_CLOUD_PASSWORD:-}"

# Parse command-line arguments
CLUSTER_PEERS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --peers)
            CLUSTER_PEERS="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --peers URLS    Comma-separated list of peer URLs (e.g., http://ip1:8484,http://ip2:8484)"
            echo "  --help, -h      Show this help message"
            echo ""
            echo "Example:"
            echo "  $0 --peers http://192.168.1.101:8484,http://192.168.1.102:8484"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

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
echo -e "│ ${YW}Grafana Cloud:${CL}   $([ -n "$GRAFANA_CLOUD_USERNAME" ] && echo "✅ Auto-configured" || echo "⚙️  Manual setup required")"
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
    "alpine")
        TEMPLATE_PATTERN="alpine-${var_version}"
        ;;
    "debian")
        TEMPLATE_PATTERN="debian-${var_version}-standard"
        ;;
    "ubuntu")
        TEMPLATE_PATTERN="ubuntu-${var_version}"
        ;;
    *)
        # Default to Alpine for smaller footprint
        TEMPLATE_PATTERN="alpine-"
        ;;
esac

# Get the latest matching template (just the filename)
OS_TEMPLATE=$(pveam list $TEMPLATE_STORAGE 2>/dev/null | grep "$TEMPLATE_PATTERN" | tail -1 | awk '{print $1}')

if [[ -z "$OS_TEMPLATE" ]]; then
    msg_error "No suitable OS template found for $var_os $var_version"
    echo "Available templates:"
    pveam list $TEMPLATE_STORAGE 2>/dev/null | grep -E "(alpine|debian|ubuntu)" | head -5
    exit 1
fi

# Handle different template extensions
TEMPLATE_NAME=$(basename "$OS_TEMPLATE" .tar.zst)
if [[ "$TEMPLATE_NAME" == "$OS_TEMPLATE" ]]; then
    # If .tar.zst didn't match, try .tar.xz (Alpine uses this)
    TEMPLATE_NAME=$(basename "$OS_TEMPLATE" .tar.xz)
fi
echo -e "${CM} ${GN}OS Template${CL}        ${TEMPLATE_NAME}"

# Create container
msg_info "Creating LXC Container"

# Build the pct create command with proper template path
# Detect the extension from the OS_TEMPLATE
if [[ "$OS_TEMPLATE" == *.tar.xz ]]; then
    TEMPLATE_PATH="${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE_NAME}.tar.xz"
else
    TEMPLATE_PATH="${TEMPLATE_STORAGE}:vztmpl/${TEMPLATE_NAME}.tar.zst"
fi

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

# Install LimeDB and OTEL Collector in container
echo ""
echo -e "📦 ${YW}Installing LimeDB + OpenTelemetry Collector...${CL}"
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"

# Note: LimeDB now requires mandatory node URL specification
# The service is configured as a single-node cluster by default
# OTEL Collector will be installed to forward telemetry to Grafana Cloud
# For multi-node clusters, edit the systemd service file after installation

# Install dependencies, LimeDB, and OTEL Collector (optimized single operation)
# Get container IP before entering container execution
CONTAINER_IP=""
if [[ -n "$NETWORK_PREFIX" ]]; then
    CONTAINER_IP="${NETWORK_PREFIX}.${CTID}"
else
    # Try to get IP from container if it's available
    CONTAINER_IP=$(pct exec $CTID -- ip route get 1 2>/dev/null | awk '{print $7}' | head -1 2>/dev/null || echo "localhost")
fi

if [[ -z "$CONTAINER_IP" || "$CONTAINER_IP" == "localhost" ]]; then
    CONTAINER_IP="localhost"
fi

# Simple peer configuration
echo ""
echo -e "${YW}Cluster Configuration${CL}"
if [[ -n "$CLUSTER_PEERS" ]]; then
    # Remove self from peers if present
    CLUSTER_PEERS=$(echo "$CLUSTER_PEERS" | sed "s|http://$CONTAINER_IP:8484||g" | sed 's/,,/,/g' | sed 's/^,//;s/,$//')
    if [[ -z "$CLUSTER_PEERS" ]]; then
        echo -e "${WARNING} ${YW}Self-references removed. Running in standalone mode.${CL}"
    else
        echo -e "${CM} ${GN}Cluster peers: $CLUSTER_PEERS${CL}"
    fi
else
    echo -e "${INFO} ${BL}No peers specified. Running in standalone mode.${CL}"
    echo -e "${DGN}Tip: Use --peers flag to configure cluster peers${CL}"
fi

echo -ne " ${YW}Installing LimeDB + OTEL Collector (all steps)...${CL}"
pct exec $CTID -- bash -c "
# Set environment variables from host
export GRAFANA_OTLP_ENDPOINT='$GRAFANA_OTLP_ENDPOINT'
export GRAFANA_CLOUD_USERNAME='$GRAFANA_CLOUD_USERNAME' 
export GRAFANA_CLOUD_PASSWORD='$GRAFANA_CLOUD_PASSWORD'
export CONTAINER_IP='$CONTAINER_IP'
export CTID='$CTID'
export CLUSTER_PEERS='$CLUSTER_PEERS'

    # Use the container IP passed from host
    if [ -z \"\$CONTAINER_IP\" ]; then
        CONTAINER_IP=\$(ip route get 1 2>/dev/null | awk '{print \$7}' | head -1)
        if [ -z \"\$CONTAINER_IP\" ]; then
            CONTAINER_IP=\"localhost\"
        fi
    fi

    # Update and install dependencies
    apt-get update &>/dev/null
    apt-get install -y curl ca-certificates wget tar &>/dev/null
    
    # Get latest version and download in parallel
    LATEST_VERSION=\$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '\"tag_name\":' | sed -E 's/.*\"([^\"]+)\".*/\1/')
    
    if [ -z \"\$LATEST_VERSION\" ]; then
        LATEST_VERSION=\"v0.0.2\"
    fi
    
    # Download LimeDB and OTEL Collector in parallel
    wget -qO /usr/local/bin/limedb \"https://github.com/namanvashistha/limedb/releases/download/\${LATEST_VERSION}/limedb-linux-amd64\" &
    LIMEDB_PID=\$!
    
    # Download OTEL Collector Contrib (includes all components)
    wget -qO /tmp/otelcol.tar.gz \"https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.140.0/otelcol-contrib_0.140.0_linux_amd64.tar.gz\" &
    OTEL_PID=\$!
    
    # Create OTEL Collector directories and config
    mkdir -p /etc/otelcol /var/log/otelcol
    
    # Create OTEL Collector configuration (minimal working configuration)
    cat > /etc/otelcol/config.yaml << OTELCONF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

  # System resource metrics
  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization:
            enabled: true
          system.cpu.time:
            enabled: true
      memory:
        metrics:
          system.memory.usage:
            enabled: true
          system.memory.utilization:
            enabled: true
      disk:
        metrics:
          system.disk.io:
            enabled: true
          system.disk.operations:
            enabled: true
          system.disk.io_time:
            enabled: true
      filesystem:
        metrics:
          system.filesystem.usage:
            enabled: true
          system.filesystem.utilization:
            enabled: true
      network:
        metrics:
          system.network.io:
            enabled: true
          system.network.packets:
            enabled: true
          system.network.errors:
            enabled: true
      process:
        metrics:
          process.cpu.utilization:
            enabled: true
          process.memory.usage:
            enabled: true
        mute_process_name_error: true
        mute_process_exe_error: true
        mute_process_io_error: true

processors:
  batch:
    timeout: 1s
    send_batch_size: 512
  resourcedetection:
    detectors: [env, system]
    override: false
  resource:
    attributes:
      - key: service.name
        value: limedb-node
        action: upsert
      - key: service.namespace
        value: limedb
        action: upsert
      - key: service.version
        value: \${LATEST_VERSION}
        action: upsert
      - key: service.instance.id
        value: \"http://\${CONTAINER_IP}:8484\"
        action: upsert
      - key: deployment.environment
        value: production
        action: upsert
      - key: host.name
        value: \"http://\${CONTAINER_IP}:8484\"
        action: upsert
      - key: node.url
        value: \"http://\${CONTAINER_IP}:8484\"
        action: upsert
      - key: limedb.node.url
        value: \"http://\${CONTAINER_IP}:8484\"
        action: upsert

exporters:
  otlphttp/grafana_cloud:
    endpoint: \"\\\${GRAFANA_OTLP_ENDPOINT}\"
    headers:
      authorization: \"Basic \\\${GRAFANA_CLOUD_AUTH_HEADER}\"
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 30s
      max_elapsed_time: 300s

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [resourcedetection, resource, batch]
      exporters: [otlphttp/grafana_cloud]
    metrics:
      receivers: [otlp, hostmetrics]
      processors: [resourcedetection, resource, batch]
      exporters: [otlphttp/grafana_cloud]
    logs:
      receivers: [otlp]
      processors: [resourcedetection, resource, batch]
      exporters: [otlphttp/grafana_cloud]
OTELCONF

    # Create OTEL environment template
    cat > /etc/otelcol/environment.template << 'ENVTEMPLATE'
# Grafana Cloud Configuration - CONFIGURE THESE VALUES
# Get these from your Grafana Cloud account:
# 1. Go to Connections -> Add new connection -> OpenTelemetry
# 2. Copy the OTLP endpoint URL (varies by region)
# 3. Generate access token with metrics and traces permissions
# 4. Use your instance ID as username and access token as password

# Examples (replace with your actual endpoints):
# US East: https://otlp-gateway-prod-us-east-0.grafana.net/otlp
# EU West: https://otlp-gateway-prod-eu-west-0.grafana.net/otlp
# US Central: https://otlp-gateway-prod-us-central-0.grafana.net/otlp
# AP South: https://otlp-gateway-prod-ap-south-1.grafana.net/otlp

GRAFANA_OTLP_ENDPOINT=your_otlp_endpoint_here
GRAFANA_CLOUD_USERNAME=your_instance_id_here
GRAFANA_CLOUD_PASSWORD=your_access_token_here
GRAFANA_CLOUD_AUTH_HEADER=your_base64_encoded_username_password_here
HOSTNAME=limedb-node
ENVTEMPLATE

    # Create default environment with dynamic values from script environment
    if [[ -n \"\$GRAFANA_CLOUD_USERNAME\" && -n \"\$GRAFANA_CLOUD_PASSWORD\" && -n \"\$GRAFANA_OTLP_ENDPOINT\" ]]; then
        # Create base64 encoded auth header
        AUTH_HEADER=\$(echo -n \"\$GRAFANA_CLOUD_USERNAME:\$GRAFANA_CLOUD_PASSWORD\" | base64 -w 0)
        
        # Use configured values
        cat > /etc/otelcol/environment << CONFIGUREDENV
GRAFANA_OTLP_ENDPOINT=\$GRAFANA_OTLP_ENDPOINT
GRAFANA_CLOUD_USERNAME=\$GRAFANA_CLOUD_USERNAME
GRAFANA_CLOUD_PASSWORD=\$GRAFANA_CLOUD_PASSWORD
GRAFANA_CLOUD_AUTH_HEADER=\$AUTH_HEADER
HOSTNAME=limedb-node
CONFIGUREDENV
    else
        # Use template values for manual configuration
        cp /etc/otelcol/environment.template /etc/otelcol/environment
    fi

    # Create OTEL Collector systemd service
    cat > /etc/systemd/system/otel-collector.service << 'OTELSVC'
[Unit]
Description=OpenTelemetry Collector
After=network.target

[Service]
Type=simple
User=root
EnvironmentFile=/etc/otelcol/environment
ExecStart=/usr/local/bin/otelcol --config=/etc/otelcol/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
OTELSVC

    # Create LimeDB systemd service with OTEL integration
    cat > /etc/systemd/system/limedb.service << LIMESVC
[Unit]
Description=LimeDB Key-Value Store
After=network.target otel-collector.service
Wants=otel-collector.service

[Service]
Type=simple
User=root
Environment=OTEL_ENABLED=true
Environment=OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
Environment=OTEL_SERVICE_NAME=limedb
Environment=OTEL_SERVICE_VERSION=\${LATEST_VERSION}
Environment=OTEL_ENVIRONMENT=production
ExecStart=/usr/local/bin/limedb -server.port 8484 -node.url \"http://\${CONTAINER_IP}:8484\"\$([ -n \"\$CLUSTER_PEERS\" ] && echo \" -node.peers \\\"\$CLUSTER_PEERS\\\"\")
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
LIMESVC
    
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
  │             + OpenTelemetry Integration             │
  │                                                     │
  ╰─────────────────────────────────────────────────────╯

  🌐 LimeDB: http://\$(ip route get 1 | awk '{print \$7}' | head -1):8484
  📊 OTEL: http://\$(ip route get 1 | awk '{print \$7}' | head -1):4317 (gRPC), :4318 (HTTP)
  📚 Documentation: https://github.com/namanvashistha/limedb
  🔧 Management: systemctl [start|stop|restart] [limedb|otel-collector]
  📋 Node URL: http://\$(ip route get 1 | awk '{print \$7}' | head -1):8484
  
  ⚙️  Configure Grafana Cloud: /etc/otelcol/environment

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
    
    # Wait for downloads to complete and install
    wait \$LIMEDB_PID
    chmod +x /usr/local/bin/limedb
    
    wait \$OTEL_PID
    cd /tmp && tar -xzf otelcol.tar.gz
    mv otelcol-contrib /usr/local/bin/otelcol
    chmod +x /usr/local/bin/otelcol
    rm -f otelcol.tar.gz
    
    # Enable services
    systemctl daemon-reload
    systemctl enable limedb.service &>/dev/null
    systemctl enable otel-collector.service &>/dev/null
    
    # Start services
    systemctl start limedb.service &>/dev/null
    
    # Start OTEL Collector if credentials are configured
    if [[ -n \"\$GRAFANA_CLOUD_USERNAME\" && -n \"\$GRAFANA_CLOUD_PASSWORD\" && -n \"\$GRAFANA_OTLP_ENDPOINT\" ]]; then
        systemctl start otel-collector.service &>/dev/null
    fi
    
    echo \"\$LATEST_VERSION\"
"
INSTALLED_VERSION=\$?
echo -e "\r${CM} ${GN}LimeDB + OTEL Collector installation completed${CL}"

# Final output
echo ""
echo -e "${DGN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${CL}"
echo -e "${SUCCESS} ${GN}LimeDB + OTEL Collector installation completed successfully!${CL}"
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
echo -e "│ ${YW}LimeDB API:${CL}     ${GATEWAY}http://${IP}:8484${CL}"
echo -e "│ ${YW}Health Check:${CL}   ${GATEWAY}curl http://${IP}:8484${CL}"
echo -e "│ ${YW}OTEL gRPC:${CL}      ${GATEWAY}http://${IP}:4317${CL}"
echo -e "│ ${YW}OTEL HTTP:${CL}      ${GATEWAY}http://${IP}:4318${CL}"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

# Management Commands
echo -e "🔧 ${YW}Management Commands${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
echo -e "│ ${DGN}Enter container:${CL}       pct enter ${CTID}"
echo -e "│ ${DGN}Stop container:${CL}        pct stop ${CTID}"
echo -e "│ ${DGN}Start container:${CL}       pct start ${CTID}"
echo -e "│ ${DGN}Restart LimeDB:${CL}        pct exec ${CTID} systemctl restart limedb"
echo -e "│ ${DGN}Restart OTEL:${CL}          pct exec ${CTID} systemctl restart otel-collector"
echo -e "│ ${DGN}View LimeDB logs:${CL}      pct exec ${CTID} journalctl -u limedb -f"
echo -e "│ ${DGN}View OTEL logs:${CL}        pct exec ${CTID} journalctl -u otel-collector -f"
echo -e "│ ${DGN}Check status:${CL}          pct exec ${CTID} systemctl status limedb otel-collector"
echo -e "└─────────────────────────────────────────────────────┘"
echo ""

# OpenTelemetry Configuration
echo -e "📊 ${YW}OpenTelemetry Configuration${CL}"
echo -e "┌─────────────────────────────────────────────────────┐"
if [[ -n "$GRAFANA_CLOUD_USERNAME" ]]; then
echo -e "│ ${YW}Status:${CL}                ✅ Pre-configured from environment"
echo -e "│ ${DGN}Start OTEL:${CL}             pct exec ${CTID} systemctl start otel-collector"
else
echo -e "│ ${YW}Status:${CL}                ⚙️  Manual configuration required"
echo -e "│ ${YW}Config File:${CL}           /etc/otelcol/environment"
echo -e "│ ${YW}Template:${CL}              /etc/otelcol/environment.template"
echo -e "│ ${DGN}Enter container:${CL}       pct enter ${CTID}"
echo -e "│ ${DGN}Edit credentials:${CL}       vi /etc/otelcol/environment"
echo -e "│ ${DGN}Start OTEL:${CL}             systemctl start otel-collector"
fi
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

# Show configuration helper information if credentials not configured
if [[ -z "$GRAFANA_CLOUD_USERNAME" ]]; then
    echo -e "💡 ${YW}Tip: Use the configuration helper for easy Grafana Cloud setup:${CL}"
    echo -e "   ${BL}bash proxmox/configure-grafana.sh${CL}"
    echo ""
fi
