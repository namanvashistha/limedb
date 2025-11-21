#!/usr/bin/env bash

# ==============================================================================
# LimeDB Grafana Cloud Configuration Helper
# ==============================================================================
# This script helps configure Grafana Cloud credentials securely in ~/.grafana/
# ==============================================================================

set -euo pipefail

# Colors
YW='\033[33m'
GN='\033[1;92m'
RD='\033[01;31m'
BL='\033[36m'
CL='\033[m'

echo -e "${GN}🔐 LimeDB Grafana Cloud Configuration Helper${CL}"
echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Function to read input securely
read_secure() {
    local prompt="$1"
    local var_name="$2"
    local is_secret="${3:-false}"
    
    echo -ne "${YW}${prompt}: ${CL}"
    if [[ "$is_secret" == "true" ]]; then
        read -s value
        echo ""
    else
        read value
    fi
    
    if [[ -z "$value" ]]; then
        echo -e "${RD}Error: Value cannot be empty${CL}"
        exit 1
    fi
    
    eval "$var_name='$value'"
}

# Check if ~/.grafana/config already exists
if [[ -f ~/.grafana/config ]]; then
    echo -e "${YW}⚠️  Existing Grafana configuration found at ~/.grafana/config${CL}"
    read -p "Do you want to overwrite it? (y/N): " overwrite
    if [[ ! "$overwrite" =~ ^[Yy]$ ]]; then
        echo -e "${BL}ℹ  Exiting without changes${CL}"
        exit 0
    fi
fi

# Get Grafana Cloud credentials
echo -e "${BL}📊 Enter your Grafana Cloud details:${CL}"
echo ""
echo -e "${YW}1. Login to Grafana Cloud${CL}"
echo -e "${YW}2. Go to Connections → Add new connection → OpenTelemetry${CL}"
echo -e "${YW}3. Copy the OTLP endpoint and generate access token${CL}"
echo ""

echo -e "${BL}📍 Common Grafana Cloud regions:${CL}"
echo -e "   US East: otlp-gateway-prod-us-east-0.grafana.net"
echo -e "   EU West: otlp-gateway-prod-eu-west-0.grafana.net"  
echo -e "   US Central: otlp-gateway-prod-us-central-0.grafana.net"
echo ""

read_secure "OTLP Endpoint (full URL)" GRAFANA_OTLP_ENDPOINT
read_secure "Instance ID (username)" GRAFANA_CLOUD_USERNAME
read_secure "Access Token (password)" GRAFANA_CLOUD_PASSWORD true

echo ""
echo -e "${GN}✅ Credentials configured successfully!${CL}"
echo ""

# Create ~/.grafana directory and config file
mkdir -p ~/.grafana
cat > ~/.grafana/config << EOF
GRAFANA_OTLP_ENDPOINT='$GRAFANA_OTLP_ENDPOINT'
GRAFANA_CLOUD_USERNAME='$GRAFANA_CLOUD_USERNAME'
GRAFANA_CLOUD_PASSWORD='$GRAFANA_CLOUD_PASSWORD'
EOF

# Set secure permissions
chmod 600 ~/.grafana/config

echo -e "${GN}✅ Configuration saved to ~/.grafana/config${CL}"
echo -e "${BL}ℹ  File secured with 600 permissions (owner read/write only)${CL}"
echo ""

# Show next steps
echo -e "${YW}📋 Next Steps:${CL}"
echo -e "1. Run the LXC setup script: ${GN}bash proxmox/limedb_lxc.sh${CL}"
echo -e "2. The script will automatically use your Grafana Cloud configuration"
echo -e "3. OTEL Collector will be pre-configured and ready to start"
echo ""

# Offer to run the script immediately
read -p "Do you want to run the LXC setup script now? (y/N): " run_script
if [[ "$run_script" =~ ^[Yy]$ ]]; then
    echo ""
    echo -e "${GN}🚀 Running LXC setup script...${CL}"
    echo ""
    
    # Check if we're in the right directory
    if [[ -f "proxmox/limedb_lxc.sh" ]]; then
        bash proxmox/limedb_lxc.sh
    else
        echo -e "${RD}Error: proxmox/limedb_lxc.sh not found${CL}"
        echo -e "${YW}Make sure you're running this from the limedb project root directory${CL}"
        exit 1
    fi
fi

echo ""
echo -e "${GN}🎉 Configuration complete!${CL}"