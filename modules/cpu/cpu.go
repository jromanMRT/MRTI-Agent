// Package cpu collects processor utilisation, frequency, core/thread counts,
// load average and (best effort) package temperature.
package cpu

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/sensors"
)

func init() { modules.Register("cpu", func() modules.Module { return &Module{} }) }

// Stats is the CPU payload.
type Stats struct {
	UsagePercent  float64   `json:"usage_percent"` // overall utilisation since last collect
	PerCore       []float64 `json:"per_core,omitempty"`
	MHz           float64   `json:"mhz,omitempty"`
	ModelName     string    `json:"model_name,omitempty"`
	PhysicalCores int       `json:"physical_cores"`
	LogicalCores  int       `json:"logical_cores"`
	Load1         float64   `json:"load1,omitempty"`
	Load5         float64   `json:"load5,omitempty"`
	Load15        float64   `json:"load15,omitempty"`
	TempCelsius   float64   `json:"temp_celsius,omitempty"`
}

type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "cpu" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	var s Stats

	// Overall percentage since the previous call (non-blocking with 0 interval).
	if overall, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(overall) > 0 {
		s.UsagePercent = round2(overall[0])
	}
	if per, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		s.PerCore = make([]float64, len(per))
		for i, v := range per {
			s.PerCore[i] = round2(v)
		}
	}

	if info, err := cpu.InfoWithContext(ctx); err == nil && len(info) > 0 {
		s.MHz = info[0].Mhz
		s.ModelName = info[0].ModelName
	}
	if n, err := cpu.CountsWithContext(ctx, false); err == nil {
		s.PhysicalCores = n
	}
	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		s.LogicalCores = n
	}

	// Load average (emulated on Windows; ignore errors there).
	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		s.Load1, s.Load5, s.Load15 = round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)
	}

	// Temperature: pick the hottest CPU-like sensor if present.
	if temps, err := sensors.TemperaturesWithContext(ctx); err == nil {
		s.TempCelsius = pickCPUTemp(temps)
	}

	return json.Marshal(s)
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

// pickCPUTemp heuristically selects a CPU package temperature from the sensor
// list. Sensor keys vary widely across platforms.
func pickCPUTemp(temps []sensors.TemperatureStat) float64 {
	var best float64
	for _, t := range temps {
		key := t.SensorKey
		if containsAny(key, "coretemp", "cpu", "k10temp", "package", "tctl", "tdie") {
			if t.Temperature > best {
				best = t.Temperature
			}
		}
	}
	return round2(best)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexFold(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// indexFold is a tiny case-insensitive substring search to avoid importing
// strings just for ToLower allocations in a hot-ish path.
func indexFold(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	lower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + 32
		}
		return b
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
