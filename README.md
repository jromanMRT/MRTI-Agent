# MRTI Agent

A professional, modular, lightweight infrastructure-monitoring agent for
**Windows** and **Ubuntu/Linux**, designed to report continuously to the
**MRTI Core** and to grow into a full fleet-management platform (à la Datadog /
Zabbix / Wazuh) — but fully owned by you.

> Status: **v0.1 foundation.** Core runtime, transport, cache, module system,
> the five core collectors and the gRPC plugin system are implemented and
> verified end-to-end on Linux and cross-compiling to Windows. The remaining
> collectors and subsystems are scaffolded with a clear path (see
> [Roadmap](#roadmap)).

---

## Highlights

- **Single self-contained binary** per platform (no runtime deps; pure-Go
  SQLite means no cgo, so cross-compiling to Windows is trivial).
- **Runs as a service** and starts at boot — systemd on Linux, Windows Service
  on Windows — via a built-in installer (`-service install`).
- **Very light**: ~20 MB RSS, ~0% idle CPU in the smoke test.
- **Modular**: collectors are enabled/disabled from config (and, in time, from
  the Core) without redeploying.
- **Extensible without recompiling the core**: new collectors ship as separate
  **gRPC plugin binaries** (HashiCorp go-plugin). A crashing plugin cannot take
  the agent down.
- **Durable delivery**: everything is buffered in a local SQLite outbox and
  retried in order across network outages and restarts.
- **Secure by default**: TLS 1.2+, optional mutual TLS / custom CA, gzip
  compression, and API-key / token / JWT authentication.

---

## Architecture

```
                         ┌─────────────────────────────────────────┐
                         │                MRTI Agent                │
                         │                                          │
  native modules ──────► │  registry ─┐                            │
  (system,cpu,ram,       │            ├─► orchestrator ─► cache ───┼──► transport ──► MRTI Core
   disk,network,…)       │  plugins ──┘   (collect loop)  (SQLite   │   (HTTPS │ WS │ MQTT)
                         │   ▲                            outbox)    │        + TLS + gzip
  gRPC plugin procs ─────┘   │                                      │        + API key/JWT
  (ping, fortigate, …)  via go-plugin                               │
                         └─────────────────────────────────────────┘
```

- **Orchestrator** (`internal/agent`) runs four independent timed loops:
  *collect* (run modules → build an envelope → enqueue), *flush* (drain the
  outbox to the server in order), *heartbeat* (cheap liveness + self-usage),
  and *commands* (poll the Core and dispatch).
- **Transport** (`internal/transport`) is an interface with a complete HTTPS
  back-end and prepared WebSocket/MQTT back-ends, selectable from config.
- **Cache** (`internal/cache`) is a bounded FIFO outbox in SQLite for
  at-least-once, ordered delivery.
- **Modules** (`modules/*`) implement one small interface. Native modules are
  compiled in; **plugins** are separate binaries adapted to the same interface
  by `internal/pluginhost`.

### Project layout

```
cmd/mrti-agent/        entrypoint, flags, service lifecycle
internal/
  agent/               orchestrator + timed loops
  config/              config.yaml loading, defaults, env overrides
  logging/             structured JSON logging with size-based rotation
  model/               wire schema (Envelope, Heartbeat, Command)
  cache/               SQLite outbox (pure-Go driver, no cgo)
  auth/                API key / token / JWT credential handling
  transport/           HTTPS (done) + WebSocket/MQTT (prepared)
  pluginhost/          loads gRPC plugins, adapts them to modules.Module
    shared/            the plugin contract (imported by agent AND plugins)
modules/               native collectors: system, cpu, ram, disk, network
proto/                 collector.proto + generated gRPC stubs
plugins/example-ping/  reference plugin (TCP latency probe)
service/               systemd unit
scripts/               install-linux.sh, install-windows.ps1
config.yaml.example    documented sample configuration
```

---

## Build

Requires **Go 1.22+**. For regenerating gRPC stubs you also need `protoc`,
`protoc-gen-go` and `protoc-gen-go-grpc` (only for `make proto`).

```bash
make build           # host binary → bin/mrti-agent
make build-linux     # → dist/linux-amd64/mrti-agent
make build-windows   # → dist/windows-amd64/mrti-agent.exe
make build-plugins   # reference ping plugin → plugins/ping
make run             # build + run in foreground with console logging
```

Cross-compilation needs no C toolchain (`CGO_ENABLED=0`).

---

## Install as a service

### Linux (systemd)

```bash
make build-linux
sudo ./scripts/install-linux.sh          # installs to /opt/mrti-agent, enables + starts
sudo systemctl status mrti-agent
journalctl -u mrti-agent -f
```

### Windows (run as Administrator)

```powershell
make build-windows        # from a build host, or build on Windows
.\scripts\install-windows.ps1 -Binary .\dist\windows-amd64\mrti-agent.exe
Get-Service mrti-agent
```

The agent can also manage its own service directly:

```
mrti-agent -service install|start|stop|restart|uninstall -config <path>
```

---

## Configuration

All behaviour is driven by `config.yaml` (see
[`config.yaml.example`](config.yaml.example) for the fully-documented version).
Key points:

- On first run the agent **generates and persists a stable `agent.id` (UUID)**.
- `server.transport` selects `https` (implemented) / `websocket` / `mqtt`.
- `modules.enabled` toggles collectors; `plugins.enabled` toggles gRPC plugins.
- Sensitive values can come from the environment: `MRTI_SERVER_URL`,
  `MRTI_API_KEY`, `MRTI_TOKEN`, `MRTI_AGENT_ID`.

Handy flags:

```
mrti-agent -foreground     # run attached with console logging (debugging)
mrti-agent -list-modules   # list built-in collectors
mrti-agent -version
```

---

## What each core module reports

| Module    | Data |
|-----------|------|
| `system`  | hostname, domain, OS/platform/version, kernel, arch, UUID, serial/model/manufacturer/BIOS (via DMI on Linux, WMI on Windows), virtualization, boot time, uptime, timezone, process count |
| `cpu`     | overall & per-core usage %, MHz, model, physical/logical cores, load average, package temperature |
| `ram`     | total/available/used/cached, used %, swap total/used/% |
| `disk`    | per-partition total/used/free/%, per-device I/O counters, SMART health (via `smartctl` when present) |
| `network` | per-interface addrs/MAC/MTU/up + traffic counters (bytes, packets, errors, drops), DNS servers, default gateway |
| `processes` | top-N by CPU/RAM (configurable `top_n`, `sort_by`): pid, name, user, cpu%, RSS, exe path, priority, threads |
| `services` | service inventory + state: systemd units (Linux) / Service Control Manager via WMI (Windows), with running/failed counts and start mode |
| `software` | installed programs + versions: dpkg/rpm (Linux) or Uninstall registry keys (Windows) |
| `docker` | containers (all): name, image, state, status, restart count, plus optional live CPU%/memory (`stats: true`); talks to the Engine API over the local socket/named-pipe |
| `eventlogs` | recent critical/error/warning entries from journald (Linux) or the Windows Event Log via Get-WinEvent; configurable `since`/`max`/`logs` |

Two subsystems complement the collectors:

- **Remote scripts** (`scripts` config): the Core issues a `run_script` command
  (bash/sh/powershell/cmd/python); the agent runs it under an interpreter
  allow-list with a timeout ceiling and returns exit code + stdout/stderr.
  Disabled by default.
- **Alerts** (`alerts` config): after each cycle the agent evaluates CPU/RAM/disk
  thresholds locally and attaches any fired alerts to the envelope as an
  `alerts` result — immediate signal without server-side rules.

Verified live payload (smoke test, Ubuntu 26.04 / KVM):

```
system : hostname=serverit, ubuntu 26.04, kernel 7.0.0-27, virt=kvm
cpu    : usage 12.5%, 12 logical cores, 4300 MHz
ram    : 45.16% used of 7.6 GB
network: 9 interfaces, gateway 192.168.1.254, dns [127.0.0.53]
plugin : ping → {"target":"8.8.8.8:53","reachable":true,"latency_ms":26.2}
```

---

## Writing a plugin (no core recompile)

A plugin is a standalone Go program that implements the tiny `Collector`
interface from `internal/pluginhost/shared` and calls `shared.Serve`:

```go
package main

import "github.com/jromanMRT/mrti-agent/internal/pluginhost/shared"

type myCollector struct{}

func (m *myCollector) Info() (string, string, string) {
    return "printers", "1.0.0", "Discovers and polls network printers"
}
func (m *myCollector) Configure(settingsJSON []byte) error { return nil }
func (m *myCollector) Collect() ([]byte, error) {
    // return any JSON payload
    return []byte(`{"printers":[]}`), nil
}

func main() { shared.Serve(&myCollector{}) }
```

Build it and drop the binary into the agent's `plugins/` directory, then enable
it:

```bash
go build -o plugins/printers ./path/to/printers
```
```yaml
plugins:
  enabled: [printers]
```

The agent launches it as a child process, speaks gRPC to it, and treats its
output exactly like a native module. See [`plugins/example-ping`](plugins/example-ping)
for a working reference.

---

## Wire protocol (to MRTI Core)

The HTTPS back-end posts to:

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/ingest` | envelopes, heartbeats and command results (`X-MRTI-Kind` header distinguishes them; body may be gzip) |
| `GET`  | `/api/v1/agents/{agentID}/commands` | pull pending commands (`204` = none) |

Auth headers: `X-MRTI-API-Key` always; `Authorization: Bearer <jwt|token>` when
configured. Schema version is carried as `schema: "mrti.v1"`.

---

## Roadmap

The foundation intentionally makes each remaining feature an additive module or
subsystem, not a rewrite.

**Implemented (v0.1 — foundation):** project skeleton · config · logging ·
SQLite outbox · auth · HTTPS transport · module registry & lifecycle ·
`system`/`cpu`/`ram`/`disk`/`network` collectors · gRPC plugin system +
reference plugin · service install (systemd/Windows) · cross-compilation.

**Implemented (v0.2 — Phase 2):** `processes`, `services` (systemd/Windows
Services), `software`/inventory collectors · **remote scripts** subsystem
(`run_script`: bash/sh/powershell/cmd/python with allow-list + timeout) ·
**alerts** subsystem (local CPU/RAM/disk threshold evaluation attached to the
envelope). All verified end-to-end.

**Implemented (v0.3 — Phase 3):** `docker` (containers, state, restart count,
optional live CPU/RAM over the Engine socket/pipe) and `eventlogs`
(journald / Windows Event Log, severity-filtered) collectors. Verified
end-to-end.

**Next collectors (native modules or plugins):** `snmp` · `ups` (NUT/APC/…) ·
`temperature` · `virtualization` detection.

**Next subsystems:**
- **Self-update** — signed binary download, version pinning, rollback.
- **More alert sources** — service-stopped, UPS battery, ping, temperature.
- **WebSocket / MQTT transports** — flesh out the prepared back-ends.
- **Central control** — Core-pushed config and runtime module toggling.

---

## License

Proprietary — © MRTI. Internal project.

_Generated with [Claude Code](https://claude.com/claude-code)._
