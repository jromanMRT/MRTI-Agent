package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// metrics exposes fleet telemetry in Prometheus text exposition format so it
// can be scraped into Grafana/VictoriaMetrics/etc. Metrics are derived from the
// latest per-module data of each agent.
func (s *server) metrics(w http.ResponseWriter, _ *http.Request) {
	var b strings.Builder
	agents := s.store.ListAgents()

	help := func(name, typ, hlp string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, hlp, name, typ)
	}
	help("mrti_agent_up", "gauge", "1 if the agent is online (seen recently)")
	help("mrti_agent_self_mem_mb", "gauge", "Agent process RSS in MB")
	help("mrti_agent_self_cpu_percent", "gauge", "Agent process CPU percent")
	help("mrti_cpu_usage_percent", "gauge", "Host CPU usage percent")
	help("mrti_mem_used_percent", "gauge", "Host RAM used percent")
	help("mrti_swap_used_percent", "gauge", "Host swap used percent")
	help("mrti_disk_used_percent", "gauge", "Filesystem used percent")
	help("mrti_temperature_celsius", "gauge", "Hottest sensor temperature")
	help("mrti_ups_battery_percent", "gauge", "UPS battery charge percent")
	help("mrti_docker_containers_running", "gauge", "Running docker containers")
	help("mrti_services_failed", "gauge", "Failed services")

	for _, a := range agents {
		lbl := fmt.Sprintf(`agent=%q,hostname=%q,os=%q`, a.ID, a.Hostname, a.OS)
		up := 0
		if a.Online {
			up = 1
		}
		fmt.Fprintf(&b, "mrti_agent_up{%s} %d\n", lbl, up)
		fmt.Fprintf(&b, "mrti_agent_self_mem_mb{%s} %g\n", lbl, a.SelfMemMB)
		fmt.Fprintf(&b, "mrti_agent_self_cpu_percent{%s} %g\n", lbl, a.SelfCPU)

		mods := s.store.modulesFor(a.ID)

		if d := mods["cpu"]; d != nil {
			var v struct {
				UsagePercent float64 `json:"usage_percent"`
			}
			if json.Unmarshal(d, &v) == nil {
				fmt.Fprintf(&b, "mrti_cpu_usage_percent{%s} %g\n", lbl, v.UsagePercent)
			}
		}
		if d := mods["ram"]; d != nil {
			var v struct {
				UsedPercent float64 `json:"used_percent"`
				SwapPercent float64 `json:"swap_used_percent"`
			}
			if json.Unmarshal(d, &v) == nil {
				fmt.Fprintf(&b, "mrti_mem_used_percent{%s} %g\n", lbl, v.UsedPercent)
				fmt.Fprintf(&b, "mrti_swap_used_percent{%s} %g\n", lbl, v.SwapPercent)
			}
		}
		if d := mods["disk"]; d != nil {
			var v struct {
				Partitions []struct {
					Mountpoint  string  `json:"mountpoint"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"partitions"`
			}
			if json.Unmarshal(d, &v) == nil {
				for _, p := range v.Partitions {
					fmt.Fprintf(&b, "mrti_disk_used_percent{%s,mount=%q} %g\n", lbl, p.Mountpoint, p.UsedPercent)
				}
			}
		}
		if d := mods["temperature"]; d != nil {
			var v struct {
				MaxCelsius float64 `json:"max_celsius"`
			}
			if json.Unmarshal(d, &v) == nil && v.MaxCelsius > 0 {
				fmt.Fprintf(&b, "mrti_temperature_celsius{%s} %g\n", lbl, v.MaxCelsius)
			}
		}
		if d := mods["ups"]; d != nil {
			var v struct {
				UPSes []struct {
					Name          string  `json:"name"`
					BatteryCharge float64 `json:"battery_charge_percent"`
				} `json:"upses"`
			}
			if json.Unmarshal(d, &v) == nil {
				for _, u := range v.UPSes {
					fmt.Fprintf(&b, "mrti_ups_battery_percent{%s,ups=%q} %g\n", lbl, u.Name, u.BatteryCharge)
				}
			}
		}
		if d := mods["docker"]; d != nil {
			var v struct {
				Running int `json:"running"`
			}
			if json.Unmarshal(d, &v) == nil {
				fmt.Fprintf(&b, "mrti_docker_containers_running{%s} %d\n", lbl, v.Running)
			}
		}
		if d := mods["services"]; d != nil {
			var v struct {
				Failed int `json:"failed"`
			}
			if json.Unmarshal(d, &v) == nil {
				fmt.Fprintf(&b, "mrti_services_failed{%s} %d\n", lbl, v.Failed)
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Write([]byte(b.String()))
}
