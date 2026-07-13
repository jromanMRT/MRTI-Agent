# Deploy on Ubuntu Server

Two ways to run MRTI on an Ubuntu host: a quick foreground test, or a permanent
systemd install of both the Core (server + API + dashboard) and the Agent.

## Prerequisites

- Go 1.22+ to build (`go version`). If Go isn't on `PATH` but was installed in a
  home dir, the installer also looks in `~/go-sdk/go/bin`.
- For the `ups` module: NUT (`upsd`) running, or set a different driver/host.

## Permanent install (systemd) — recommended

From the repo root, as root:

```bash
make build build-core build-plugins      # or the installer builds them for you
sudo ./scripts/install-server.sh
```

This installs to `/opt/mrti`, generates a shared API key (`/opt/mrti/api-key`),
writes `/opt/mrti/agent.yaml`, and enables + starts two services:

- **mrti-core.service** — the server on `:8477` (dashboard, API, `/metrics`)
- **mrti-agent.service** — the agent monitoring this host, reporting to the Core

Then:

```
Dashboard : http://<server-ip>:8477/
API       : http://<server-ip>:8477/api/v1/agents
Metrics   : http://<server-ip>:8477/metrics

systemctl status mrti-core mrti-agent
journalctl -u mrti-agent -f
```

Re-running the installer is safe — it preserves the API key, agent id and any
edits to `agent.yaml`.

### Let other machines report here

```bash
sudo ufw allow 8477/tcp          # open the port
```

On each other host (Ubuntu or Windows) install just the **agent** with
`server.url: http://<server-ip>:8477` and the **same API key** from
`/opt/mrti/api-key`. They'll appear in the dashboard/API within seconds.

## Quick foreground test (no install)

```bash
make build build-core build-plugins
./bin/mrti-core -addr :8477 -db core.db -api-key demo-key &   # server
./bin/mrti-agent -foreground -config run/agent.yaml           # agent (edit api_key to match)
```

## Uninstall

```bash
sudo systemctl disable --now mrti-agent mrti-core
sudo rm /etc/systemd/system/mrti-{agent,core}.service
sudo systemctl daemon-reload
sudo rm -rf /opt/mrti
```

## Notes

- The Core is a single-node reference server (SQLite, latest-state). For large
  fleets, put TLS in front (reverse proxy) and scrape `/metrics` into Prometheus.
- Some fields need privileges: disk SMART needs `smartctl`; hardware serial via
  DMI needs root. The systemd agent runs as root by default, so these populate.
