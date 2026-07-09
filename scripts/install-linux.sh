#!/usr/bin/env bash
# Install the MRTI Agent as a systemd service on Ubuntu/Debian.
# Usage: sudo ./scripts/install-linux.sh [/path/to/mrti-agent-binary]
set -euo pipefail

INSTALL_DIR="/opt/mrti-agent"
BINARY="${1:-dist/linux-amd64/mrti-agent}"

if [[ $EUID -ne 0 ]]; then
  echo "This installer must run as root (use sudo)." >&2
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  echo "Agent binary not found at '$BINARY'. Build it first: make build-linux" >&2
  exit 1
fi

echo "==> Installing MRTI Agent to $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"/{logs,cache,plugins}
install -m 0755 "$BINARY" "$INSTALL_DIR/mrti-agent"

# Preserve an existing config; otherwise drop the example in place.
if [[ ! -f "$INSTALL_DIR/config.yaml" ]]; then
  install -m 0640 config.yaml.example "$INSTALL_DIR/config.yaml"
  echo "==> Installed default config at $INSTALL_DIR/config.yaml — EDIT server.url and credentials."
fi

echo "==> Installing systemd unit"
install -m 0644 service/mrti-agent.service /etc/systemd/system/mrti-agent.service

systemctl daemon-reload
systemctl enable mrti-agent.service
systemctl restart mrti-agent.service

echo "==> Done. Status:"
systemctl --no-pager status mrti-agent.service || true
echo
echo "Logs: journalctl -u mrti-agent -f   (and $INSTALL_DIR/logs/)"
