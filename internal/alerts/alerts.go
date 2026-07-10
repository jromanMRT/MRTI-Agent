// Package alerts performs lightweight, local threshold evaluation right after
// each collection cycle. Detecting "CPU high", "disk full", etc. on the agent
// means the Core is notified immediately — even between metric batches — and
// operators get signal without server-side rules for the common cases.
package alerts

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// Severity levels.
const (
	Warning  = "warning"
	Critical = "critical"
)

// Alert is a single fired rule.
type Alert struct {
	Rule      string    `json:"rule"`
	Severity  string    `json:"severity"`
	Resource  string    `json:"resource"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
}

// Evaluate inspects a cycle's module results against the configured thresholds
// and returns any alerts that fired. It decodes only the fields it needs, so it
// stays decoupled from the module packages.
func Evaluate(cfg config.AlertsConfig, results []model.ModuleResult) []Alert {
	if !cfg.Enabled {
		return nil
	}
	now := time.Now()
	var out []Alert

	for _, r := range results {
		if r.Error != "" || len(r.Data) == 0 {
			continue
		}
		switch r.Module {
		case "cpu":
			var d struct {
				UsagePercent float64 `json:"usage_percent"`
			}
			if json.Unmarshal(r.Data, &d) == nil && cfg.CPUPercent > 0 && d.UsagePercent >= cfg.CPUPercent {
				out = append(out, Alert{
					Rule: "cpu_high", Severity: sev(d.UsagePercent, cfg.CPUPercent),
					Resource: "cpu", Value: d.UsagePercent, Threshold: cfg.CPUPercent, Timestamp: now,
					Message: fmt.Sprintf("CPU usage %.1f%% ≥ %.0f%%", d.UsagePercent, cfg.CPUPercent),
				})
			}
		case "ram":
			var d struct {
				UsedPercent float64 `json:"used_percent"`
			}
			if json.Unmarshal(r.Data, &d) == nil && cfg.MemPercent > 0 && d.UsedPercent >= cfg.MemPercent {
				out = append(out, Alert{
					Rule: "memory_high", Severity: sev(d.UsedPercent, cfg.MemPercent),
					Resource: "ram", Value: d.UsedPercent, Threshold: cfg.MemPercent, Timestamp: now,
					Message: fmt.Sprintf("RAM usage %.1f%% ≥ %.0f%%", d.UsedPercent, cfg.MemPercent),
				})
			}
		case "disk":
			var d struct {
				Partitions []struct {
					Mountpoint  string  `json:"mountpoint"`
					UsedPercent float64 `json:"used_percent"`
				} `json:"partitions"`
			}
			if json.Unmarshal(r.Data, &d) == nil && cfg.DiskPercent > 0 {
				for _, p := range d.Partitions {
					if p.UsedPercent >= cfg.DiskPercent {
						out = append(out, Alert{
							Rule: "disk_full", Severity: sev(p.UsedPercent, cfg.DiskPercent),
							Resource: "disk:" + p.Mountpoint, Value: p.UsedPercent,
							Threshold: cfg.DiskPercent, Timestamp: now,
							Message: fmt.Sprintf("Disk %s at %.1f%% ≥ %.0f%%", p.Mountpoint, p.UsedPercent, cfg.DiskPercent),
						})
					}
				}
			}
		case "ups":
			var d struct {
				UPSes []struct {
					Name          string  `json:"name"`
					Status        string  `json:"status"`
					BatteryCharge float64 `json:"battery_charge_percent"`
				} `json:"upses"`
			}
			if json.Unmarshal(r.Data, &d) == nil {
				for _, u := range d.UPSes {
					if u.Status == "on_battery" || u.Status == "low_battery" {
						out = append(out, Alert{
							Rule: "ups_on_battery", Severity: pick(u.Status == "low_battery", Critical, Warning),
							Resource: "ups:" + u.Name, Value: u.BatteryCharge, Timestamp: now,
							Message: fmt.Sprintf("UPS %s is %s (battery %.0f%%)", u.Name, u.Status, u.BatteryCharge),
						})
					} else if cfg.UPSBattery > 0 && u.BatteryCharge > 0 && u.BatteryCharge < cfg.UPSBattery {
						out = append(out, Alert{
							Rule: "ups_battery_low", Severity: Warning,
							Resource: "ups:" + u.Name, Value: u.BatteryCharge, Threshold: cfg.UPSBattery, Timestamp: now,
							Message: fmt.Sprintf("UPS %s battery %.0f%% < %.0f%%", u.Name, u.BatteryCharge, cfg.UPSBattery),
						})
					}
				}
			}
		case "temperature":
			var d struct {
				MaxCelsius float64 `json:"max_celsius"`
				HottestKey string  `json:"hottest_key"`
			}
			if json.Unmarshal(r.Data, &d) == nil && cfg.TempCelsius > 0 && d.MaxCelsius >= cfg.TempCelsius {
				out = append(out, Alert{
					Rule: "temperature_high", Severity: sev(d.MaxCelsius, cfg.TempCelsius),
					Resource: "temp:" + d.HottestKey, Value: d.MaxCelsius, Threshold: cfg.TempCelsius, Timestamp: now,
					Message: fmt.Sprintf("Sensor %s at %.1f°C ≥ %.0f°C", d.HottestKey, d.MaxCelsius, cfg.TempCelsius),
				})
			}
		case "ping":
			// Emitted by the reference ping plugin: {target,reachable,latency_ms}.
			var d struct {
				Target    string  `json:"target"`
				Reachable bool    `json:"reachable"`
				LatencyMS float64 `json:"latency_ms"`
			}
			if json.Unmarshal(r.Data, &d) == nil {
				if !d.Reachable {
					out = append(out, Alert{
						Rule: "host_unreachable", Severity: Critical,
						Resource: "ping:" + d.Target, Timestamp: now,
						Message: fmt.Sprintf("%s is unreachable", d.Target),
					})
				} else if cfg.PingLatencyMS > 0 && d.LatencyMS >= cfg.PingLatencyMS {
					out = append(out, Alert{
						Rule: "ping_high", Severity: Warning,
						Resource: "ping:" + d.Target, Value: d.LatencyMS, Threshold: cfg.PingLatencyMS, Timestamp: now,
						Message: fmt.Sprintf("%s latency %.0fms ≥ %.0fms", d.Target, d.LatencyMS, cfg.PingLatencyMS),
					})
				}
			}
		case "services":
			if !cfg.ServiceStopped {
				continue
			}
			var d struct {
				Services []struct {
					Name  string `json:"name"`
					State string `json:"state"`
				} `json:"services"`
			}
			if json.Unmarshal(r.Data, &d) == nil {
				for _, s := range d.Services {
					if s.State == "failed" {
						out = append(out, Alert{
							Rule: "service_failed", Severity: Warning,
							Resource: "service:" + s.Name, Timestamp: now,
							Message: fmt.Sprintf("Service %s is in failed state", s.Name),
						})
					}
				}
			}
		}
	}
	return out
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// sev escalates to critical once usage is well past the threshold.
func sev(value, threshold float64) string {
	if value >= threshold+((100-threshold)/2) || value >= 98 {
		return Critical
	}
	return Warning
}
