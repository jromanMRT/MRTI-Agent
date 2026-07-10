#!/usr/bin/env python3
"""Mock MRTI Core for the local demo.

Receives the agent's telemetry (envelopes, heartbeats, command results) and
prints a readable live report to the terminal. It also pushes two commands to
the agent — a `ping` and a `run_script` (uptime) — to demonstrate the full
Core -> agent -> result round trip.

Run:  python3 demo/mock-core.py
"""
import gzip
import json
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = 8477
commands_sent = {"done": False}


def hr():
    return "─" * 72


def fmt_bytes(n):
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024:
            return f"{n:.0f}{unit}"
        n /= 1024
    return f"{n:.0f}PB"


def summarize(results):
    by = {r["module"]: r for r in results}

    def data(name):
        r = by.get(name)
        return r.get("data") if r and isinstance(r.get("data"), dict) else None

    lines = []
    d = data("system")
    if d:
        lines.append(f"  system     : {d.get('hostname')} · {d.get('platform')} {d.get('version')} · "
                     f"kernel {d.get('kernel')} · virt={d.get('virtualization') or 'bare-metal'}")
    d = data("cpu")
    if d:
        lines.append(f"  cpu        : {d.get('usage_percent')}% used · {d.get('logical_cores')} cores · "
                     f"{d.get('mhz')}MHz · load {d.get('load1')}")
    d = data("ram")
    if d:
        lines.append(f"  ram        : {d.get('used_percent')}% of {fmt_bytes(d.get('total_bytes',0))} · "
                     f"swap {d.get('swap_used_percent')}%")
    d = data("disk")
    if d:
        parts = d.get("partitions", [])
        worst = max(parts, key=lambda p: p.get("used_percent", 0), default=None)
        extra = f" · busiest {worst['mountpoint']} {worst['used_percent']}%" if worst else ""
        lines.append(f"  disk       : {len(parts)} partitions{extra}")
    d = data("network")
    if d:
        lines.append(f"  network    : {len(d.get('interfaces',[]))} interfaces · gw {d.get('gateway')} · "
                     f"dns {d.get('dns')}")
    d = data("processes")
    if d:
        top = d.get("top", [])
        t = top[0] if top else {}
        lines.append(f"  processes  : {d.get('total')} total · top {t.get('name')} {t.get('cpu_percent')}%CPU")
    d = data("services")
    if d:
        lines.append(f"  services   : {d.get('total')} total · {d.get('running')} running · {d.get('failed')} failed")
    d = data("software")
    if d:
        lines.append(f"  software   : {d.get('total')} packages ({d.get('source')})")
    d = data("docker")
    if d:
        if d.get("available"):
            lines.append(f"  docker     : {d.get('running')}/{d.get('total')} containers running")
        else:
            lines.append("  docker     : no daemon")
    d = data("eventlogs")
    if d:
        lines.append(f"  eventlogs  : {d.get('criticals')} crit · {d.get('errors')} err · {d.get('warnings')} warn ({d.get('source')})")
    d = data("temperature")
    if d:
        lines.append(f"  temperature: {len(d.get('sensors',[]))} sensors · hottest {d.get('hottest_key')} {d.get('max_celsius')}°C")
    d = data("virtualization")
    if d:
        role = "hypervisor" if d.get("is_hypervisor") else ("VM/container" if d.get("is_virtual") else "bare-metal")
        lines.append(f"  virtual.   : {d.get('system') or 'none'} · {role}")
    d = data("inventory")
    if d:
        lines.append(f"  inventory  : {len(d.get('gpus',[]))} GPU · {len(d.get('pci',[]))} PCI · "
                     f"{len(d.get('usb',[]))} USB · {len(d.get('monitors',[]))} monitors")
    d = data("ups")
    if d:
        if d.get("available") and d.get("upses"):
            u = d["upses"][0]
            lines.append(f"  ups        : {u.get('name')} {u.get('status')} · batt {u.get('battery_charge_percent')}% · "
                         f"{u.get('input_voltage')}V in · load {u.get('load_percent')}%")
        else:
            lines.append(f"  ups        : unavailable ({d.get('error','')[:40]})")
    d = data("ping")
    if d:
        lines.append(f"  ping(plugin): {d.get('target')} reachable={d.get('reachable')} {d.get('latency_ms')}ms")

    # Alerts are a synthetic result carrying a list.
    ar = by.get("alerts")
    if ar and isinstance(ar.get("data"), list):
        for a in ar["data"]:
            lines.append(f"  ⚠ ALERT    : [{a['severity']}] {a['rule']} — {a['message']}")
    return "\n".join(lines)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(n)
        if self.headers.get("Content-Encoding") == "gzip":
            body = gzip.decompress(body)
        kind = self.headers.get("X-MRTI-Kind", "?")
        msg = json.loads(body.decode())
        ts = datetime.now().strftime("%H:%M:%S")

        if kind == "heartbeat":
            print(f"[{ts}] 💓 heartbeat  agent={msg.get('hostname')} v{msg.get('version')} "
                  f"{msg.get('os')}/{msg.get('arch')} · modules={len(msg.get('active_modules',[]))} · "
                  f"self {msg.get('self_mem_mb')}MB / {msg.get('self_cpu_percent')}%CPU")
        elif kind == "envelope":
            print(f"\n[{ts}] 📦 ENVELOPE #{msg.get('sequence')}  ({len(msg.get('results',[]))} modules)")
            print(summarize(msg.get("results", [])))
            print(hr())
        elif kind == "command_result":
            ok = "✅" if msg.get("ok") else "❌"
            out = msg.get("output", "")
            try:
                parsed = json.loads(out)
                if "stdout" in parsed:
                    out = "stdout=" + repr(parsed["stdout"][:80])
                else:
                    out = json.dumps(parsed)[:100]
            except Exception:
                pass
            print(f"[{ts}] {ok} command_result id={msg.get('command_id')} · {out} {msg.get('error','')}")

        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

    def do_GET(self):
        # Push two commands once, to demonstrate the command channel.
        if "/commands" in self.path and not commands_sent["done"]:
            commands_sent["done"] = True
            cmds = [
                {"id": "demo-ping", "type": "ping"},
                {"id": "demo-script", "type": "run_script",
                 "payload": {"interpreter": "bash", "script": "echo demo-from-core; uptime", "timeout_seconds": 10}},
            ]
            data = json.dumps(cmds).encode()
            print(f"\n[{datetime.now():%H:%M:%S}] ⬇  pushing {len(cmds)} commands to the agent (ping + run_script)\n")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        self.send_response(204)
        self.end_headers()


if __name__ == "__main__":
    print(hr())
    print("  MRTI mock Core — listening on http://127.0.0.1:%d" % PORT)
    print("  Waiting for the agent… (Ctrl-C to stop)")
    print(hr())
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
