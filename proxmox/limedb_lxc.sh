#!/usr/bin/env bash

set -euo pipefail

# Colors
YW='\033[33m'; GN='\033[1;92m'; RD='\033[01;31m'; CL='\033[m'

# Configuration
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-150}"
var_disk="${var_disk:-2}"
PASSWORD="password"

# Grafana Cloud (optional)
GRAFANA_CONFIG_DIR="$HOME/.grafana"
[[ -f "$GRAFANA_CONFIG_DIR/config" ]] && source "$GRAFANA_CONFIG_DIR/config"
GRAFANA_OTLP_ENDPOINT="${GRAFANA_OTLP_ENDPOINT:-}"
GRAFANA_CLOUD_USERNAME="${GRAFANA_CLOUD_USERNAME:-}"
GRAFANA_CLOUD_PASSWORD="${GRAFANA_CLOUD_PASSWORD:-}"

# Parse arguments
CLUSTER_PEERS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --peers) CLUSTER_PEERS="$2"; shift 2 ;;
        --help|-h) echo "Usage: $0 [--peers URLs]"; exit 0 ;;
        *) echo "Unknown: $1"; exit 1 ;;
    esac
done

# Validate
[[ $EUID -ne 0 ]] && { echo -e "${RD}Run as root${CL}"; exit 1; }
command -v pct &>/dev/null || { echo -e "${RD}Proxmox not found${CL}"; exit 1; }

echo -e "${YW}LimeDB Container Setup${CL}\n"

# Find available CTID
CTID=100
while pct status $CTID &>/dev/null; do ((CTID++)); done
echo "Container ID: $CTID"

# Detect storage
TEMPLATE_STORAGE=$(pvesm status -content vztmpl 2>/dev/null | awk 'NR>1 && /active/ {print $1; exit}')
CONTAINER_STORAGE=$(pvesm status -content rootdir 2>/dev/null | awk 'NR>1 && /active/ {print $1; exit}')
[[ -z "$TEMPLATE_STORAGE" ]] && TEMPLATE_STORAGE="local"
[[ -z "$CONTAINER_STORAGE" ]] && CONTAINER_STORAGE="local-lvm"

# Find Alpine template
OS_TEMPLATE=$(pveam list $TEMPLATE_STORAGE 2>/dev/null | grep "alpine-3" | tail -1 | awk '{print $1}')
[[ -z "$OS_TEMPLATE" ]] && { echo -e "${RD}No Alpine template found${CL}"; exit 1; }
TEMPLATE_PATH="/var/lib/vz/template/cache/$(basename ${OS_TEMPLATE##*:vztmpl/})"

# Network config
HOST_IP=$(ip route get 1 2>/dev/null | awk '{print $7}' | head -1)
if [[ -n "$HOST_IP" ]]; then
    NETWORK_PREFIX=$(echo "$HOST_IP" | cut -d. -f1-3)
    GATEWAY_IP=$(ip route show default 2>/dev/null | awk '{print $3}' | head -1)
    [[ -z "$GATEWAY_IP" ]] && GATEWAY_IP="${NETWORK_PREFIX}.1"
    NET_CONFIG="ip=${NETWORK_PREFIX}.${CTID}/24,gw=${GATEWAY_IP}"
    IP="${NETWORK_PREFIX}.${CTID}"
else
    NET_CONFIG="dhcp"
    IP="dhcp"
fi

# Create container
echo "Creating container..."
pct create $CTID "$TEMPLATE_PATH" \
    --hostname limedb \
    --cores $var_cpu \
    --memory $var_ram \
    --swap 512 \
    --storage $CONTAINER_STORAGE \
    --password $PASSWORD \
    --net0 name=eth0,bridge=vmbr0,${NET_CONFIG} \
    --nameserver 8.8.8.8 \
    --features nesting=1 \
    --unprivileged 1 \
    --rootfs $CONTAINER_STORAGE:${var_disk} || { echo -e "${RD}Failed to create${CL}"; exit 1; }

# Start container
pct start $CTID || { echo -e "${RD}Failed to start${CL}"; exit 1; }

# Wait for container
echo "Waiting for container..."
for i in {1..30}; do
    pct exec $CTID -- test -d /etc &>/dev/null && break
    sleep 1
done

# Install
echo "Installing LimeDB..."
CONTAINER_IP="$IP"
[[ "$IP" == "dhcp" ]] && CONTAINER_IP=$(pct exec $CTID -- ip route get 1 2>/dev/null | awk '{print $7}' | head -1)

pct exec $CTID -- sh -c "
export GRAFANA_OTLP_ENDPOINT='$GRAFANA_OTLP_ENDPOINT'
export GRAFANA_CLOUD_USERNAME='$GRAFANA_CLOUD_USERNAME'
export GRAFANA_CLOUD_PASSWORD='$GRAFANA_CLOUD_PASSWORD'

apk update &>/dev/null
apk add --no-cache curl wget tar bash &>/dev/null

LATEST_VERSION=\$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '\"tag_name\":' | sed -E 's/.*\"([^\"]+)\".*/\1/')
[[ -z \"\$LATEST_VERSION\" ]] && LATEST_VERSION=\"v0.0.2\"

wget -qO /usr/local/bin/limedb \"https://github.com/namanvashistha/limedb/releases/download/\${LATEST_VERSION}/limedb-linux-amd64\"
chmod +x /usr/local/bin/limedb

wget -qO /tmp/otel.tar.gz \"https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.140.0/otelcol-contrib_0.140.0_linux_amd64.tar.gz\"
tar -xzf /tmp/otel.tar.gz -C /tmp
mv /tmp/otelcol-contrib /usr/local/bin/otelcol
chmod +x /usr/local/bin/otelcol
rm -f /tmp/otel.tar.gz

mkdir -p /etc/otelcol /var/log/otelcol

cat > /etc/otelcol/config.yaml << 'OTELCFG'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
processors:
  batch:
exporters:
  otlphttp/grafana_cloud:
    endpoint: \"\\\${GRAFANA_OTLP_ENDPOINT}\"
    headers:
      authorization: \"Basic \\\${GRAFANA_CLOUD_AUTH_HEADER}\"
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/grafana_cloud]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/grafana_cloud]
OTELCFG

if [[ -n \"\$GRAFANA_CLOUD_USERNAME\" && -n \"\$GRAFANA_CLOUD_PASSWORD\" ]]; then
    AUTH_HEADER=\$(echo -n \"\$GRAFANA_CLOUD_USERNAME:\$GRAFANA_CLOUD_PASSWORD\" | base64 -w 0)
    cat > /etc/otelcol/environment << ENV
GRAFANA_OTLP_ENDPOINT=\$GRAFANA_OTLP_ENDPOINT
GRAFANA_CLOUD_AUTH_HEADER=\$AUTH_HEADER
ENV
else
    echo 'GRAFANA_OTLP_ENDPOINT=' > /etc/otelcol/environment
    echo 'GRAFANA_CLOUD_AUTH_HEADER=' >> /etc/otelcol/environment
fi

cat > /etc/init.d/otel-collector << 'OTELRC'
#!/sbin/openrc-run
name=\"OTEL Collector\"
command=\"/usr/local/bin/otelcol\"
command_args=\"--config=/etc/otelcol/config.yaml\"
command_background=true
pidfile=\"/var/run/otelcol.pid\"
output_log=\"/var/log/otelcol/otelcol.log\"
depend() { need net; }
start_pre() {
    [[ -f /etc/otelcol/environment ]] && . /etc/otelcol/environment
    mkdir -p /var/log/otelcol
}
OTELRC
chmod +x /etc/init.d/otel-collector

PEERS_ARG=''
[[ -n '$CLUSTER_PEERS' ]] && PEERS_ARG=' -node.peers \"$CLUSTER_PEERS\"'

cat > /etc/init.d/limedb << LIMERC
#!/sbin/openrc-run
name=\"LimeDB\"
command=\"/usr/local/bin/limedb\"
command_args=\"-server.port 8484 -node.url \\\"http://$CONTAINER_IP:8484\\\"\$PEERS_ARG\"
command_background=true
pidfile=\"/var/run/limedb.pid\"
output_log=\"/var/log/limedb.log\"
depend() { need net; }
start_pre() {
    export OTEL_ENABLED=true
    export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
}
LIMERC
chmod +x /etc/init.d/limedb

rc-update add limedb default &>/dev/null
rc-service limedb start &>/dev/null

[[ -n \"\$GRAFANA_CLOUD_USERNAME\" ]] && {
    rc-update add otel-collector default &>/dev/null
    rc-service otel-collector start &>/dev/null
}

sed -i 's/^tty1/#tty1/' /etc/inittab 2>/dev/null || true
echo 'tty1::respawn:/sbin/getty -n -l /bin/sh 38400 tty1' >> /etc/inittab
"

echo -e "\n${GN}✓ Installation complete${CL}\n"
echo "Container ID: $CTID"
echo "IP Address: $CONTAINER_IP"
echo "Password: $PASSWORD"
echo "LimeDB: http://$CONTAINER_IP:8484"
echo ""
echo "Commands:"
echo "  pct enter $CTID"
echo "  pct exec $CTID rc-service limedb restart"
echo "  pct exec $CTID tail -f /var/log/limedb.log"
echo ""
