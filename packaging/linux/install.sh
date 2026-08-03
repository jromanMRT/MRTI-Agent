#!/usr/bin/env bash
# Install MRTI Agent as a systemd service. Run from the extracted package.
set -euo pipefail

INSTALL_DIR="/opt/mrti-agent"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Ejecuta este instalador como administrador: sudo ./install.sh" >&2
  exit 1
fi

[[ -f "$SCRIPT_DIR/mrti-agent" ]] || { echo "No se encontró mrti-agent." >&2; exit 1; }
[[ -f "$SCRIPT_DIR/config.yaml" ]] || { echo "No se encontró config.yaml." >&2; exit 1; }

install -d "$INSTALL_DIR" "$INSTALL_DIR/logs" "$INSTALL_DIR/cache" "$INSTALL_DIR/plugins"
install -m 0755 "$SCRIPT_DIR/mrti-agent" "$INSTALL_DIR/mrti-agent"
if [[ ! -f "$INSTALL_DIR/config.yaml" ]]; then
  install -m 0600 "$SCRIPT_DIR/config.yaml" "$INSTALL_DIR/config.yaml"
fi

cat > /etc/systemd/system/mrti-agent.service <<EOF
[Unit]
Description=MRTI infrastructure monitoring agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/mrti-agent -config ${INSTALL_DIR}/config.yaml
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=5
Nice=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now mrti-agent.service

echo "MRTI Agent instalado y ejecutándose en segundo plano."
echo "Estado: systemctl status mrti-agent"
echo "Logs:   journalctl -u mrti-agent -f"
