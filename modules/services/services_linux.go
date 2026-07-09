//go:build linux

package services

import (
	"context"
	"encoding/json"
	"os/exec"
)

// listServices queries systemd via `systemctl` JSON output. It merges the
// loaded unit list (active/sub state) with the unit-file list (enablement =
// start mode). Absence of systemd yields an empty list rather than an error.
func listServices(ctx context.Context) ([]Service, error) {
	units := loadedUnits(ctx)
	modes := unitFileModes(ctx)

	out := make([]Service, 0, len(units))
	for name, u := range units {
		svc := Service{
			Name:  name,
			State: normalizeState(u.Active, u.Sub),
			Sub:   u.Sub,
		}
		if mode, ok := modes[name]; ok {
			svc.StartMode = mode
		}
		out = append(out, svc)
	}
	return out, nil
}

type unitState struct {
	Active string
	Sub    string
}

// loadedUnits parses `systemctl list-units --type=service`.
func loadedUnits(ctx context.Context) map[string]unitState {
	res := map[string]unitState{}
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service",
		"--all", "--no-pager", "--output=json")
	data, err := cmd.Output()
	if err != nil {
		return res
	}
	var rows []struct {
		Unit   string `json:"unit"`
		Active string `json:"active"`
		Sub    string `json:"sub"`
	}
	if json.Unmarshal(data, &rows) != nil {
		return res
	}
	for _, r := range rows {
		res[r.Unit] = unitState{Active: r.Active, Sub: r.Sub}
	}
	return res
}

// unitFileModes parses `systemctl list-unit-files` for enablement state.
func unitFileModes(ctx context.Context) map[string]string {
	res := map[string]string{}
	cmd := exec.CommandContext(ctx, "systemctl", "list-unit-files", "--type=service",
		"--no-pager", "--output=json")
	data, err := cmd.Output()
	if err != nil {
		return res
	}
	var rows []struct {
		UnitFile string `json:"unit_file"`
		State    string `json:"state"`
	}
	if json.Unmarshal(data, &rows) != nil {
		return res
	}
	for _, r := range rows {
		switch r.State {
		case "enabled", "enabled-runtime", "static", "generated":
			res[r.UnitFile] = "auto"
		case "disabled":
			res[r.UnitFile] = "disabled"
		default:
			res[r.UnitFile] = "manual"
		}
	}
	return res
}

// normalizeState maps systemd active/sub states to the common vocabulary.
func normalizeState(active, sub string) string {
	switch active {
	case "active":
		if sub == "running" {
			return "running"
		}
		return "running" // active but exited (e.g. oneshot) still counts as up
	case "failed":
		return "failed"
	case "inactive", "deactivating":
		return "stopped"
	default:
		return "unknown"
	}
}
