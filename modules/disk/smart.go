package disk

import (
	"context"
	"encoding/json"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// SMARTInfo is a condensed SMART health record for one device.
type SMARTInfo struct {
	Device       string `json:"device"`
	Model        string `json:"model,omitempty"`
	Serial       string `json:"serial,omitempty"`
	Passed       bool   `json:"passed"` // overall self-assessment
	TempCelsius  int    `json:"temp_celsius,omitempty"`
	PowerOnHours int    `json:"power_on_hours,omitempty"`
}

var (
	smartOnce sync.Once
	smartPath string // resolved path to smartctl, "" if unavailable
)

// smartctlAvailable resolves the smartctl binary once. SMART is a value-add,
// not a requirement, so absence is handled silently.
func smartctlAvailable() string {
	smartOnce.Do(func() {
		if p, err := exec.LookPath("smartctl"); err == nil {
			smartPath = p
		}
	})
	return smartPath
}

// collectSMART enumerates devices via `smartctl --scan-open` and pulls a
// health summary for each. Requires smartmontools; often needs root. Any error
// yields a nil slice so the disk module keeps working regardless.
func collectSMART(ctx context.Context, log *slog.Logger) []SMARTInfo {
	bin := smartctlAvailable()
	if bin == "" {
		return nil
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var scan struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}
	if out, err := exec.CommandContext(scanCtx, bin, "--scan-open", "--json").Output(); err == nil {
		_ = json.Unmarshal(out, &scan)
	}

	var results []SMARTInfo
	for _, d := range scan.Devices {
		devCtx, dcancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := exec.CommandContext(devCtx, bin, "-a", "--json", "-d", d.Type, d.Name).Output()
		dcancel()
		if err != nil && len(out) == 0 {
			continue
		}
		var info struct {
			ModelName    string `json:"model_name"`
			SerialNumber string `json:"serial_number"`
			SmartStatus  struct {
				Passed bool `json:"passed"`
			} `json:"smart_status"`
			Temperature struct {
				Current int `json:"current"`
			} `json:"temperature"`
			PowerOnTime struct {
				Hours int `json:"hours"`
			} `json:"power_on_time"`
		}
		if err := json.Unmarshal(out, &info); err != nil {
			continue
		}
		results = append(results, SMARTInfo{
			Device:       d.Name,
			Model:        info.ModelName,
			Serial:       info.SerialNumber,
			Passed:       info.SmartStatus.Passed,
			TempCelsius:  info.Temperature.Current,
			PowerOnHours: info.PowerOnTime.Hours,
		})
	}
	return results
}
