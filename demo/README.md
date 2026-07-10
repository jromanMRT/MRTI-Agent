# MRTI Agent — local demo

Run the agent end-to-end on your own machine against a **mock Core** that prints
a live report of everything the agent collects and does — no MRTI server needed.

## One command

```bash
./demo/run-demo.sh
```

This builds the agent + the `ping` plugin, starts the mock Core
(`mock-core.py`) on `http://127.0.0.1:8477`, and runs the agent in the
foreground. Press **Ctrl-C** to stop both.

Requires: **Go 1.22+** and **python3** (standard library only). If `go` isn't on
your `PATH`, the script also looks in `~/go-sdk/go/bin`.

## What you'll see

Every few seconds the mock Core prints:

- a **💓 heartbeat** line (agent identity, active module count, the agent's own
  CPU/RAM footprint), and
- a **📦 ENVELOPE** with a one-line summary per module — system, cpu, ram, disk,
  network, processes, services, software, docker, eventlogs, temperature,
  virtualization, inventory, ups, plus the `ping` plugin — and any **⚠ ALERT**
  that fired.

It also pushes two commands once at startup to show the Core→agent→result loop:

- `ping` → the agent replies `pong`
- `run_script` → the agent runs `uptime` under bash and returns stdout

Example (abridged):

```
[10:08:39] ✅ command_result id=demo-ping · pong
[10:08:39] ✅ command_result id=demo-script · stdout='demo-from-core\n 10:08:37 up 2 days...'

[10:08:39] 📦 ENVELOPE #1  (16 modules)
  system     : serverit · ubuntu 26.04 · kernel 7.0.0-27 · virt=kvm
  cpu        : 18.18% used · 12 cores · load 0.29
  ram        : 40.92% of 7GB
  docker     : 5/5 containers running
  temperature: 16 sensors · hottest pch_cometlake 44°C
  inventory  : 1 GPU · 17 PCI · 4 USB · 3 monitors
  ups        : mrtups online · batt 100% · 126V in
  ⚠ ALERT    : [warning] temperature_high — Sensor pch_cometlake at 44.0°C ≥ 40°C
```

## Customising

Edit [`config.demo.yaml`](config.demo.yaml):

- Enable `snmp` and add `targets` to poll network devices.
- Change `ups.driver` to `apc` (with host/port) for an apcupsd server.
- Tune `alerts.*` thresholds to make alerts fire (or not) on your host.
- Add your own gRPC plugin binary under `plugins/` and list it in `plugins.enabled`.

## Notes

- Some collectors are richer with privileges: `disk` SMART needs `smartctl`
  (and often root); hardware serials via DMI may need root. Run with `sudo` to
  see those fields populated.
- The agent auto-generates and persists an `agent.id` on first run — the demo
  uses a throwaway copy of the config so your repo stays clean.
