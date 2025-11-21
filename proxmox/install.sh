#!/usr/bin/env bash

# Colors and functions (minimal versions for the install script)
YW=$(echo "\033[33m")
RD=$(echo "\033[01;31m")
GN=$(echo "\033[1;92m")
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

# Silent execution helper
STD="&>/dev/null"
[ "$VERBOSE" == "yes" ] && STD=""

msg_info "Installing Dependencies"
eval "apt-get update $STD"
eval "apt-get install -y curl ca-certificates wget $STD"
msg_ok "Installed Dependencies"

msg_info "Installing LimeDB"
# Get latest version
LATEST_VERSION=$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
  LATEST_VERSION="v0.0.2"  # Fallback version
fi

eval "wget -qO /usr/local/bin/limedb 'https://github.com/namanvashistha/limedb/releases/download/${LATEST_VERSION}/limedb-linux-amd64' $STD"
eval "chmod +x /usr/local/bin/limedb $STD"
msg_ok "Installed LimeDB ${LATEST_VERSION}"

msg_info "Creating Service"
cat <<EOF >/etc/systemd/system/limedb.service
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

eval "systemctl daemon-reload $STD"
eval "systemctl enable --now limedb.service $STD"
msg_ok "Created Service"

msg_info "Setting up MOTD"
cat <<'EOF' >/etc/motd
   __    _                ____  ____
  / /   (_)___ ___  ___  / __ \/ __ )
 / /   / / __ `__ \/ _ \/ / / / __  |
/ /___/ / / / / / /  __/ /_/ / /_/ /
/_____/_/_/ /_/ /_/\___/_____/_____/

LimeDB - Fast Key-Value Store
Access: http://YOUR_IP:7001
Docs: https://github.com/namanvashistha/limedb

EOF
msg_ok "Setup Complete"