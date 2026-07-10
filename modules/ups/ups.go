// Package ups reports UPS (uninterruptible power supply) status. It supports
// pluggable back-end drivers; the NUT (Network UPS Tools) driver is fully
// implemented and speaks the upsd TCP protocol natively (no external client
// binary). APC (apcupsd) and SNMP UPS drivers are stubbed for later.
package ups

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("ups", func() modules.Module { return &Module{} }) }

// UPSInfo is one UPS device, normalised across drivers.
type UPSInfo struct {
	Name           string            `json:"name"`
	Manufacturer   string            `json:"manufacturer,omitempty"`
	Model          string            `json:"model,omitempty"`
	Serial         string            `json:"serial,omitempty"`
	Status         string            `json:"status"`               // online | on_battery | low_battery | charging | unknown
	StatusRaw      string            `json:"status_raw,omitempty"` // driver-native status (e.g. NUT "OL CHRG")
	BatteryCharge  float64           `json:"battery_charge_percent"`
	BatteryRuntime int               `json:"battery_runtime_seconds"`
	BatteryVoltage float64           `json:"battery_voltage,omitempty"`
	InputVoltage   float64           `json:"input_voltage,omitempty"`
	OutputVoltage  float64           `json:"output_voltage,omitempty"`
	Load           float64           `json:"load_percent,omitempty"`
	Temperature    float64           `json:"temperature,omitempty"`
	Raw            map[string]string `json:"raw,omitempty"`
}

// Stats is the ups payload.
type Stats struct {
	Available bool      `json:"available"`
	Driver    string    `json:"driver"`
	UPSes     []UPSInfo `json:"upses,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// driver is the abstraction each UPS back-end implements.
type driver interface {
	query(ctx context.Context) ([]UPSInfo, error)
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
	driverName string
	drv        driver
}

func (m *Module) Name() string { return "ups" }

// Configure selects the driver from settings. Supported: "nut" (default).
func (m *Module) Configure(settings map[string]any, log *slog.Logger) error {
	m.BaseModule.Configure(settings, log)
	m.driverName = "nut"
	if v, ok := settings["driver"].(string); ok && v != "" {
		m.driverName = v
	}
	switch m.driverName {
	case "nut":
		m.drv = newNUTDriver(settings)
	case "apc":
		m.drv = newAPCDriver(settings)
	default:
		// Unknown/unimplemented driver: Collect reports it rather than crashing.
		m.drv = nil
	}
	return nil
}

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	if m.drv == nil {
		return json.Marshal(Stats{
			Available: false, Driver: m.driverName,
			Error: fmt.Sprintf("ups driver %q not implemented", m.driverName),
		})
	}
	upses, err := m.drv.query(ctx)
	if err != nil {
		return json.Marshal(Stats{Available: false, Driver: m.driverName, Error: err.Error()})
	}
	return json.Marshal(Stats{Available: true, Driver: m.driverName, UPSes: upses})
}
