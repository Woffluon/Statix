#!/usr/bin/env bash
set -euo pipefail

# Statix One-Command Installer
# Usage: curl -sSL https://raw.githubusercontent.com/Woffluon/Statix/main/deploy/install.sh | bash

REPO="Woffluon/Statix"
INSTALL_BIN="/usr/local/bin/statix"
CONFIG_DIR="/etc/statix"
SERVICE_FILE="/etc/systemd/system/statix.service"

echo "==> Starting Statix installation..."

# Ensure running as root
if [ "$(id -u)" -ne 0 ]; then
  echo "Error: install.sh must be run as root (use sudo)." >&2
  exit 1
fi

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture ${ARCH}." >&2
    exit 1
    ;;
esac

echo "==> Detected architecture: linux/${ARCH}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

BINARY_URL="https://github.com/${REPO}/releases/latest/download/statix-linux-${ARCH}"
CHECKSUM_URL="${BINARY_URL}.sha256"

echo "==> Downloading latest binary from ${BINARY_URL}..."
if ! curl -sSL "${BINARY_URL}" -o "${TMP_DIR}/statix"; then
  echo "Warning: Download from release failed (possibly dev build). Checking local bin/ or fallback..."
  if [ -f "./bin/statix-linux-${ARCH}" ]; then
    cp "./bin/statix-linux-${ARCH}" "${TMP_DIR}/statix"
  else
    echo "Error: Could not obtain Statix binary." >&2
    exit 1
  fi
fi

chmod +x "${TMP_DIR}/statix"
echo "==> Installing binary to ${INSTALL_BIN}..."
mv "${TMP_DIR}/statix" "${INSTALL_BIN}"

# Create statix system user if missing
if ! id -u statix >/dev/null 2>&1; then
  echo "==> Creating statix system user..."
  useradd --system --no-create-home --shell /usr/sbin/nologin statix
fi

# Create config directory with 0700 permissions
echo "==> Creating configuration directory ${CONFIG_DIR}..."
mkdir -p "${CONFIG_DIR}"
chown -R statix:statix "${CONFIG_DIR}"
chmod 0700 "${CONFIG_DIR}"

# Install systemd unit
echo "==> Installing systemd service unit..."
cat <<'EOF' > "${SERVICE_FILE}"
[Unit]
Description=Statix System Resource Monitor
After=network.target

[Service]
Type=simple
User=statix
Group=statix
ExecStart=/usr/local/bin/statix --config /etc/statix/config.yaml
Restart=on-failure
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/etc/statix
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF

chmod 0644 "${SERVICE_FILE}"

echo "==> Enabling and starting Statix service..."
systemctl daemon-reload
systemctl enable statix
systemctl restart statix

echo "==> Statix installation complete!"
echo "==> Access the setup wizard at http://<your-server-ip>:8080"
echo "==> (If port 8080 was busy, Statix automatically bound to the next free port."
echo "==>  Check active port with: journalctl -u statix -n 20)"
