// Package eventlogs reports recent critical/error/warning events from the
// host's log system: the systemd journal on Linux and the Windows Event Log on
// Windows. It surfaces only elevated-severity entries within a recent window so
// payloads stay small and actionable.
package eventlogs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("eventlogs", func() modules.Module { return &Module{} }) }

// Event is one normalised log entry.
type Event struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"` // critical | error | warning
	Source  string    `json:"source,omitempty"`
	Unit    string    `json:"unit,omitempty"`
	Message string    `json:"message"`
}

// Stats is the eventlogs payload.
type Stats struct {
	Source    string  `json:"source"` // journald | windows-eventlog
	Window    string  `json:"window"`
	Criticals int     `json:"criticals"`
	Errors    int     `json:"errors"`
	Warnings  int     `json:"warnings"`
	Events    []Event `json:"events"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
	since time.Duration
	max   int
	logs  []string // Windows log names (System, Application, ...)
}

func (m *Module) Name() string { return "eventlogs" }

// Configure reads optional settings: since (Go duration, default 1h), max
// (default 100) and logs (Windows log names, default System+Application).
func (m *Module) Configure(settings map[string]any, log *slog.Logger) error {
	m.BaseModule.Configure(settings, log)
	m.since = time.Hour
	m.max = 100
	m.logs = []string{"System", "Application"}

	if v, ok := settings["since"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			m.since = d
		}
	}
	if v, ok := settings["max"]; ok {
		if n, ok := toInt(v); ok && n > 0 {
			m.max = n
		}
	}
	if v, ok := settings["logs"].([]any); ok && len(v) > 0 {
		var names []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		if len(names) > 0 {
			m.logs = names
		}
	}
	return nil
}

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	source, events, err := collectEvents(ctx, m.since, m.max, m.logs)
	if err != nil {
		return nil, err
	}
	stats := Stats{Source: source, Window: m.since.String(), Events: events}
	for _, e := range events {
		switch e.Level {
		case "critical":
			stats.Criticals++
		case "error":
			stats.Errors++
		case "warning":
			stats.Warnings++
		}
	}
	return json.Marshal(stats)
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
