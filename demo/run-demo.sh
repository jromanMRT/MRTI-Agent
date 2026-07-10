#!/usr/bin/env bash
# One-command local demo of the MRTI Agent.
#
#   ./demo/run-demo.sh
#
# Builds the agent + ping plugin, starts the mock Core (which prints a live
# report of everything the agent sends), and runs the agent in the foreground.
# Press Ctrl-C to stop both.
set -euo pipefail
cd "$(dirname "$0")/.."

# Locate a Go toolchain (falls back to a home-dir install if not on PATH).
if command -v go >/dev/null 2>&1; then
  GO=go
elif [ -x "$HOME/go-sdk/go/bin/go" ]; then
  GO="$HOME/go-sdk/go/bin/go"
else
  echo "Go toolchain not found. Install Go 1.22+ and re-run." >&2
  exit 1
fi

echo "==> Building agent and ping plugin…"
"$GO" build -o bin/mrti-agent ./cmd/mrti-agent
"$GO" build -o plugins/ping ./plugins/example-ping

mkdir -p demo/logs demo/cache
rm -f demo/cache/demo.db* demo/config.demo.local.yaml
# Use a throwaway copy so the agent's auto-saved agent-id doesn't dirty the repo.
cp demo/config.demo.yaml demo/config.demo.local.yaml

echo "==> Starting mock Core…"
python3 demo/mock-core.py &
CORE_PID=$!
cleanup() { kill "$CORE_PID" 2>/dev/null || true; rm -f demo/config.demo.local.yaml; }
trap cleanup EXIT INT TERM
sleep 1

echo "==> Starting agent (Ctrl-C to stop)…"
echo
./bin/mrti-agent -foreground -config demo/config.demo.local.yaml
