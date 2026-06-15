#!/usr/bin/env bash
# install.sh — build and deploy hivemind-gw + OpenRC service
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINARY_SRC="${REPO_ROOT}/hivemind-gw"
BINARY_DEST="/usr/local/bin/hivemind-gw"
SERVICE_SRC="${REPO_ROOT}/etc/openrc/hivemind"
SERVICE_DEST="/etc/init.d/hivemind"
CONFIG_DIR="/etc/hivemind"
LOG_DIR="/var/log/hivemind"

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: must run as root" >&2
    exit 1
fi

echo "==> Building hivemind-gw..."
cd "${REPO_ROOT}"
go build -o hivemind-gw ./cmd/hivemind-gw/...

echo "==> Installing binary to ${BINARY_DEST}..."
install -m 0755 "${BINARY_SRC}" "${BINARY_DEST}"

echo "==> Installing OpenRC service..."
install -m 0755 "${SERVICE_SRC}" "${SERVICE_DEST}"

echo "==> Creating config directory ${CONFIG_DIR}..."
install -d -m 0750 "${CONFIG_DIR}"
if [ ! -f "${CONFIG_DIR}/env" ]; then
    touch "${CONFIG_DIR}/env"
    chmod 0640 "${CONFIG_DIR}/env"
    echo "# Hivemind environment variables" > "${CONFIG_DIR}/env"
    echo "# HIVEMIND_ADDR=:8080" >> "${CONFIG_DIR}/env"
fi

echo "==> Creating log directory ${LOG_DIR}..."
install -d -m 0755 "${LOG_DIR}"

echo "==> Adding hivemind to default runlevel..."
rc-update add hivemind default

echo ""
echo "Done. Edit ${CONFIG_DIR}/env then run: rc-service hivemind start"
