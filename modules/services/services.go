// Package services reports the host's service inventory and state: systemd
// units on Linux and the Service Control Manager (via WMI) on Windows. This is
// what powers "service stopped" alerting and remote start/stop later.
package services

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("services", func() modules.Module { return &Module{} }) }

// Service is one service/unit entry, normalised across platforms.
type Service struct {
	Name      string `json:"name"`
	Display   string `json:"display,omitempty"`
	State     string `json:"state"`                // running | stopped | failed | unknown
	Sub       string `json:"sub,omitempty"`        // systemd sub-state (running/exited/dead)
	StartMode string `json:"start_mode,omitempty"` // auto | manual | disabled
	PID       int32  `json:"pid,omitempty"`
}

// Stats is the services payload.
type Stats struct {
	Total    int       `json:"total"`
	Running  int       `json:"running"`
	Failed   int       `json:"failed"`
	Services []Service `json:"services"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "services" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	list, err := listServices(ctx)
	if err != nil {
		return nil, err
	}
	stats := Stats{Total: len(list), Services: list}
	for _, s := range list {
		switch s.State {
		case "running":
			stats.Running++
		case "failed":
			stats.Failed++
		}
	}
	return json.Marshal(stats)
}
