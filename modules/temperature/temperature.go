// Package temperature reports all thermal sensors exposed by the host: CPU
// package/cores, chipset, NVMe, ACPI zones, etc. It uses gopsutil's sensors
// package, which reads hwmon on Linux and WMI thermal zones on Windows.
package temperature

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/sensors"
)

func init() { modules.Register("temperature", func() modules.Module { return &Module{} }) }

// Sensor is one thermal reading.
type Sensor struct {
	Key      string  `json:"key"`
	Celsius  float64 `json:"celsius"`
	High     float64 `json:"high,omitempty"`
	Critical float64 `json:"critical,omitempty"`
}

// Stats is the temperature payload.
type Stats struct {
	Sensors    []Sensor `json:"sensors"`
	MaxCelsius float64  `json:"max_celsius"`
	HottestKey string   `json:"hottest_key,omitempty"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "temperature" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	temps, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		// Some hosts (many VMs) expose no sensors; that's not a failure.
		return json.Marshal(Stats{})
	}

	stats := Stats{}
	for _, t := range temps {
		if t.Temperature == 0 {
			continue
		}
		s := Sensor{
			Key:      t.SensorKey,
			Celsius:  round2(t.Temperature),
			High:     round2(t.High),
			Critical: round2(t.Critical),
		}
		stats.Sensors = append(stats.Sensors, s)
		if s.Celsius > stats.MaxCelsius {
			stats.MaxCelsius = s.Celsius
			stats.HottestKey = s.Key
		}
	}
	return json.Marshal(stats)
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
