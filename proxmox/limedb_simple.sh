#!/usr/bin/env bash
source <(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/misc/build.func)
# Copyright (c) 2025 namanvashistha
# Author: namanvashistha
# License: MIT
# Source: https://github.com/namanvashistha/limedb

APP="LimeDB"
var_tags="${var_tags:-database;keyvalue}"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-512}"
var_disk="${var_disk:-2}"
var_os="${var_os:-debian}"
var_version="${var_version:-12}"
var_unprivileged="${var_unprivileged:-1}"

header_info "$APP"
variables
color
catch_errors

function update_script() {
  header_info
  check_container_storage
  check_container_resources

  if ! command -v limedb >/dev/null 2>&1; then
    msg_error "No ${APP} Installation Found!"
    exit 1
  fi

  msg_info "Updating LimeDB LXC"
  
  # Get latest version
  LATEST_VERSION=$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  
  if [ -z "$LATEST_VERSION" ]; then
    msg_error "Failed to get latest version"
    exit 1
  fi

  # Stop service
  $STD systemctl stop limedb
  
  # Download and install latest version
  $STD wget -qO /usr/local/bin/limedb "https://github.com/namanvashistha/limedb/releases/download/${LATEST_VERSION}/limedb-linux-amd64"
  $STD chmod +x /usr/local/bin/limedb
  
  # Start service
  $STD systemctl start limedb
  
  msg_ok "Updated LimeDB to ${LATEST_VERSION}"
  exit
}

start
build_container
description

msg_info "Installing Dependencies"
$STD apt-get update
$STD apt-get install -y curl ca-certificates wget
msg_ok "Installed Dependencies"

msg_info "Installing LimeDB"
# Get latest version
LATEST_VERSION=$(curl -s https://api.github.com/repos/namanvashistha/limedb/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
  LATEST_VERSION="v0.0.2"  # Fallback version
fi

$STD wget -qO /usr/local/bin/limedb "https://github.com/namanvashistha/limedb/releases/download/${LATEST_VERSION}/limedb-linux-amd64"
$STD chmod +x /usr/local/bin/limedb
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

$STD systemctl daemon-reload
$STD systemctl enable --now limedb.service
msg_ok "Created Service"

motd_ssh
customize

clean

msg_ok "Completed Successfully!\n"
echo -e "${CREATING}${GN}${APP} setup has been successfully initialized!${CL}"
echo -e "${INFO}${YW} Access it using the following URL:${CL}"
echo -e "${TAB}${GATEWAY}${BGN}http://${IP}:7001${CL}"