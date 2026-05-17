#!/usr/bin/env bash
# Simulated 1Panel node: agent + sshd. The agent needs the same
# runtime config the master does (mode=dev app.yaml + a 1pctl params
# stub that common.LoadParams greps). Before enrollment it runs as a
# local master (unix socket); after the master pushes
# /etc/1panel/bootstrap/* and bounces it, it comes up in mTLS slave
# mode on 0.0.0.0:9999.
set -euo pipefail

INSTALL_DIR="/opt"
CONF_DIR="${INSTALL_DIR}/1panel/conf"
mkdir -p "${CONF_DIR}" /etc/1panel

if [ ! -f "${CONF_DIR}/app.yaml" ]; then
cat > "${CONF_DIR}/app.yaml" <<EOF
base:
  install_dir: ${INSTALL_DIR}
  mode: dev
  is_demo: false
  is_offline: false
  is_fxplay: false
  port: 9999
  version: v2.0.0
log:
  level: info
  time_zone: UTC
  log_name: 1Panel
  log_suffix: .log
  max_backup: 10
EOF
fi

if [ ! -f /usr/local/bin/1pctl ] || ! grep -q '^BASE_DIR=' /usr/local/bin/1pctl; then
cat > /usr/local/bin/1pctl <<EOF
#!/bin/sh
BASE_DIR=${INSTALL_DIR}
ORIGINAL_PORT=9999
ORIGINAL_VERSION=v2.0.0
ORIGINAL_USERNAME=admin
ORIGINAL_PASSWORD=admin123
ORIGINAL_ENTRANCE=entrance
LANGUAGE=en
PANEL_EDITION=community
EOF
chmod 0755 /usr/local/bin/1pctl
fi

# sshd so the master can push the bootstrap bundle.
/usr/sbin/sshd
echo "[node] sshd up; root password = nodepass123"

# Supervisor: relaunch the agent whenever it exits (the systemctl shim
# kills it on enrollment so it re-reads the bootstrap dir).
#
# OSS slave-transition gotcha: server.go (slave branch) loads
# ServerCrt/ServerKey from the setting table, which is populated by the
# InitSetting migration. If the agent already initialised in MASTER
# mode, gormigrate marks InitSetting applied and never re-runs it in
# slave mode, so the bootstrap certs are never persisted ->
# "tls: failed to find any PEM data" -> exit loop. A freshly-installed
# agent (never run as master) doesn't hit this. We reproduce that:
# the first time a slave bootstrap appears, drop the master-mode DBs so
# InitSetting re-runs in slave mode and persists the pushed certs.
(
  while true; do
    if [ -f /etc/1panel/bootstrap/scope ] && [ ! -f /opt/1panel/.slave_reset ]; then
      echo "[node] slave bootstrap detected — resetting master-mode DB so slave certs persist"
      rm -rf /opt/1panel/db
      touch /opt/1panel/.slave_reset
    fi
    /usr/local/bin/1panel-agent || true
    echo "[node] 1panel-agent exited, relaunching in 1s"
    sleep 1
  done
) &
SUP=$!

trap 'kill "${SUP}" 2>/dev/null || true; pkill -x 1panel-agent 2>/dev/null || true' TERM INT
wait "${SUP}"
