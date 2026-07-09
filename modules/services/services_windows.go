//go:build windows

package services

import (
	"context"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

// listServices queries the Windows Service Control Manager via WMI. The wmi
// package is already a transitive dependency of gopsutil on Windows.
func listServices(_ context.Context) ([]Service, error) {
	var rows []struct {
		Name        string
		DisplayName string
		State       string // Running | Stopped | ...
		StartMode   string // Auto | Manual | Disabled
		ProcessId   uint32
	}
	q := "SELECT Name, DisplayName, State, StartMode, ProcessId FROM Win32_Service"
	if err := wmi.Query(q, &rows); err != nil {
		return nil, err
	}

	out := make([]Service, 0, len(rows))
	for _, r := range rows {
		out = append(out, Service{
			Name:      r.Name,
			Display:   r.DisplayName,
			State:     normalizeState(r.State),
			StartMode: normalizeMode(r.StartMode),
			PID:       int32(r.ProcessId),
		})
	}
	return out, nil
}

func normalizeState(s string) string {
	switch strings.ToLower(s) {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	default:
		return "unknown"
	}
}

func normalizeMode(s string) string {
	switch strings.ToLower(s) {
	case "auto":
		return "auto"
	case "manual":
		return "manual"
	case "disabled":
		return "disabled"
	default:
		return s
	}
}
