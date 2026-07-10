# MRTI Core — reference server & API

`cmd/mrti-core` is a self-hostable reference server that receives telemetry from
MRTI agents, stores the latest state per agent in SQLite, and exposes it as a
JSON REST API, a Prometheus `/metrics` endpoint and a live HTML dashboard. Point
an agent's `server.url` at it and the agent appears in the API within seconds.

## Run

```bash
make build-core
./bin/mrti-core -addr :8477 -db core.db -api-key demo-api-key
```

Flags: `-addr` (listen address), `-db` (SQLite path), `-api-key` (the key agents
must send as `X-MRTI-API-Key` on ingest; read endpoints are open).

Then open the dashboard at <http://localhost:8477/>.

## Endpoints

### Agent-facing (used by the agent's transport)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/ingest` | receive envelopes/heartbeats/command results (`X-MRTI-Kind`; gzip aware; auth via `X-MRTI-API-Key`) |
| `GET`  | `/api/v1/agents/{id}/commands` | deliver queued commands (`204` when none) |

### Operator API (JSON)

| Method | Path | Returns |
|--------|------|---------|
| `GET`  | `/api/v1/agents` | list of agents with online status, version, self-usage, last sequence |
| `GET`  | `/api/v1/agents/{id}` | the agent's latest full envelope (all modules) |
| `GET`  | `/api/v1/agents/{id}/modules/{module}` | one module's latest data (e.g. `docker`, `ups`, `inventory`) |
| `GET`  | `/api/v1/alerts?limit=N` | recent alerts across the fleet |
| `GET`  | `/api/v1/export` | full fleet dump as a downloadable JSON file |
| `GET`  | `/metrics` | Prometheus text exposition (cpu/mem/disk/temp/ups/docker/…) |
| `POST` | `/api/v1/agents/{id}/commands` | enqueue a command for an agent |
| `GET`  | `/healthz` | liveness |

## Examples

```bash
# List agents
curl -s http://localhost:8477/api/v1/agents | jq

# One module of one agent
curl -s http://localhost:8477/api/v1/agents/<id>/modules/docker | jq

# Export the whole fleet to a file
curl -s http://localhost:8477/api/v1/export > mrti-export.json

# Scrape metrics (point Prometheus/Grafana here)
curl -s http://localhost:8477/metrics
```

### Sending a command to an agent

The agent polls `/commands` and executes what it finds. Enqueue one via:

```bash
# Run a remote script (requires scripts.enabled on the agent)
curl -X POST http://localhost:8477/api/v1/agents/<id>/commands \
  -H 'Content-Type: application/json' \
  -d '{"type":"run_script","payload":{"interpreter":"bash","script":"uptime"}}'

# Toggle a module live
curl -X POST http://localhost:8477/api/v1/agents/<id>/commands \
  -d '{"type":"enable_module","payload":{"module":"snmp"}}'

# Push a whole new config (hot-reloads what it can, restarts if needed)
curl -X POST http://localhost:8477/api/v1/agents/<id>/commands \
  -d '{"type":"set_config","payload":{"yaml":"<full config.yaml as a string>"}}'
```

Supported command types: `ping`, `run_script`, `update`, `enable_module`,
`disable_module`, `set_config`.

## Prometheus / Grafana

`/metrics` exposes per-agent gauges labelled with `agent`, `hostname`, `os`:

```
mrti_agent_up{...} 1
mrti_cpu_usage_percent{...} 3.29
mrti_mem_used_percent{...} 42.08
mrti_disk_used_percent{...,mount="/"} 21.06
mrti_temperature_celsius{...} 44
mrti_ups_battery_percent{...,ups="mrtups"} 100
mrti_docker_containers_running{...} 5
mrti_services_failed{...} 0
```

Add a scrape job pointing at `host:8477` and build dashboards as usual.

## Notes

This is a **reference** Core: single-node, SQLite-backed, latest-state (not a
time-series database). It's ideal for a homelab, a small fleet, or as the
foundation to grow into the full MRTI platform. For large fleets, front it with
TLS (a reverse proxy) and scrape `/metrics` into a proper TSDB.
