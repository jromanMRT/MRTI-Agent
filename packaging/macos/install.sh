#!/usr/bin/env bash
# Install MRTI Agent as a system-wide launchd service.
set -euo pipefail

INSTALL_DIR="/Library/Application Support/MRTI Agent"
PLIST="/Library/LaunchDaemons/com.mrti.agent.plist"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [[ $(id -u) -ne 0 ]]; then
  echo "Ejecuta este instalador como administrador: sudo ./install.sh" >&2
  exit 1
fi

[[ -f "$SCRIPT_DIR/mrti-agent" ]] || { echo "No se encontró mrti-agent." >&2; exit 1; }
[[ -f "$SCRIPT_DIR/config.yaml" ]] || { echo "No se encontró config.yaml." >&2; exit 1; }

mkdir -p "$INSTALL_DIR/logs" "$INSTALL_DIR/cache" "$INSTALL_DIR/plugins"
install -m 0755 "$SCRIPT_DIR/mrti-agent" "$INSTALL_DIR/mrti-agent"
if [[ ! -f "$INSTALL_DIR/config.yaml" ]]; then
  install -m 0600 "$SCRIPT_DIR/config.yaml" "$INSTALL_DIR/config.yaml"
fi

cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.mrti.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/mrti-agent</string>
    <string>-config</string>
    <string>${INSTALL_DIR}/config.yaml</string>
  </array>
  <key>WorkingDirectory</key><string>${INSTALL_DIR}</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>${INSTALL_DIR}/logs/launchd.log</string>
  <key>StandardErrorPath</key><string>${INSTALL_DIR}/logs/launchd-error.log</string>
</dict>
</plist>
EOF

chmod 0644 "$PLIST"
chown root:wheel "$PLIST"
launchctl bootout system/com.mrti.agent >/dev/null 2>&1 || true
launchctl bootstrap system "$PLIST"
launchctl enable system/com.mrti.agent
launchctl kickstart -k system/com.mrti.agent

echo "MRTI Agent instalado y ejecutándose en segundo plano."
echo "Estado: launchctl print system/com.mrti.agent"
echo "Logs:   $INSTALL_DIR/logs/"
