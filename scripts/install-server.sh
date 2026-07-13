#!/usr/bin/env bash
# Install BOTH the MRTI Core server and the MRTI Agent as systemd services on
# this Ubuntu/Debian host. The agent monitors this machine and reports to the
# local Core; other machines can point their agents at http://THIS_HOST:8477.
#
# Usage:  sudo ./scripts/install-server.sh
#
# Re-running is safe: it preserves the existing API key, agent id and config.
set -euo pipefail

INSTALL_DIR="/opt/mrti"
PORT="${MRTI_PORT:-8477}"
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [[ $EUID -ne 0 ]]; then
  echo "This installer must run as root:  sudo $0" >&2
  exit 1
fi

# --- locate a Go toolchain and build if the binaries aren't present ----------
find_go() {
  if command -v go >/dev/null 2>&1; then echo go; return; fi
  for h in /root /home/*; do
    [[ -x "$h/go-sdk/go/bin/go" ]] && { echo "$h/go-sdk/go/bin/go"; return; }
  done
  echo ""
}

need_build=0
[[ -x "$REPO_DIR/bin/mrti-core" ]]  || need_build=1
[[ -x "$REPO_DIR/bin/mrti-agent" ]] || need_build=1
[[ -x "$REPO_DIR/plugins/ping" ]]   || need_build=1

if [[ $need_build -eq 1 ]]; then
  GO="$(find_go)"
  [[ -n "$GO" ]] || { echo "Go not found and binaries missing. Run 'make build build-core build-plugins' first." >&2; exit 1; }
  echo "==> Building binaries with $GO"
  ( cd "$REPO_DIR" && "$GO" build -trimpath -ldflags "-s -w" -o bin/mrti-core ./cmd/mrti-core \
      && "$GO" build -trimpath -ldflags "-s -w" -o bin/mrti-agent ./cmd/mrti-agent \
      && "$GO" build -trimpath -o plugins/ping ./plugins/example-ping )
fi

echo "==> Installing to $INSTALL_DIR"
install -d "$INSTALL_DIR" "$INSTALL_DIR/logs" "$INSTALL_DIR/cache" "$INSTALL_DIR/plugins"
install -m 0755 "$REPO_DIR/bin/mrti-core"  "$INSTALL_DIR/mrti-core"
install -m 0755 "$REPO_DIR/bin/mrti-agent" "$INSTALL_DIR/mrti-agent"
install -m 0755 "$REPO_DIR/plugins/ping"   "$INSTALL_DIR/plugins/ping"

# --- API key (generated once, shared by Core and agent) ----------------------
KEY_FILE="$INSTALL_DIR/api-key"
if [[ ! -f "$KEY_FILE" ]]; then
  head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32 > "$KEY_FILE"
  chmod 600 "$KEY_FILE"
fi
API_KEY="$(cat "$KEY_FILE")"

# --- agent config (preserve existing so agent id / edits survive) ------------
AGENT_CFG="$INSTALL_DIR/agent.yaml"
if [[ ! -f "$AGENT_CFG" ]]; then
  cat > "$AGENT_CFG" <<EOF
agent:
  name: "$(hostname)"
  tags: ["ubuntu-server"]
server:
  url: "http://127.0.0.1:${PORT}"
  transport: "https"
  api_key: "${API_KEY}"
  compression: true
  tls: { insecure_skip_verify: true }
intervals: { collect: 15s, heartbeat: 30s, flush: 10s, commands: 15s }
logging: { level: "info", dir: "${INSTALL_DIR}/logs", console: false }
cache: { path: "${INSTALL_DIR}/cache/agent.db", max_queue: 5000 }
alerts:
  enabled: true
  cpu_percent: 90
  mem_percent: 90
  disk_percent: 85
  temp_celsius: 75
  ups_battery: 50
modules:
  enabled: [system, cpu, ram, disk, network, processes, services, software,
            docker, eventlogs, temperature, virtualization, inventory, ups]
  config:
    processes: { top_n: 10, sort_by: cpu }
    eventlogs: { since: "24h", max: 50 }
    ups: { driver: nut, host: 127.0.0.1, port: 3493 }
plugins:
  dir: "${INSTALL_DIR}/plugins"
  enabled: [ping]
EOF
  chmod 600 "$AGENT_CFG"
fi

# --- systemd units -----------------------------------------------------------
cat > /etc/systemd/system/mrti-core.service <<EOF
[Unit]
Description=MRTI Core - fleet telemetry server + API + dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/mrti-core -addr :${PORT} -db ${INSTALL_DIR}/core.db -api-key ${API_KEY}
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=5
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/mrti-agent.service <<EOF
[Unit]
Description=MRTI Agent - infrastructure monitoring agent
After=network-online.target mrti-core.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/mrti-agent -config ${INSTALL_DIR}/agent.yaml
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=5
Nice=10

[Install]
WantedBy=multi-user.target
EOF

echo "==> Enabling and starting services"
systemctl daemon-reload
systemctl enable --now mrti-core.service
sleep 1
systemctl enable --now mrti-agent.service

IP="$(hostname -I | awk '{print $1}')"
echo
echo "==> Done."
echo "    Dashboard : http://${IP}:${PORT}/"
echo "    API       : http://${IP}:${PORT}/api/v1/agents"
echo "    Metrics   : http://${IP}:${PORT}/metrics"
echo "    API key   : ${API_KEY}   (also in ${KEY_FILE})"
echo
echo "    Status : systemctl status mrti-core mrti-agent"
echo "    Logs   : journalctl -u mrti-core -f   /   journalctl -u mrti-agent -f"
echo
echo "    To let OTHER machines report here, open the port:"
echo "        sudo ufw allow ${PORT}/tcp"
echo "    then install the agent on them with server.url=http://${IP}:${PORT} and the API key above."
