// Package processes reports the top processes by resource usage: pid, name,
// owner, cpu%, memory, executable path and priority. Reporting only the top-N
// (configurable) keeps payloads small on busy hosts while still surfacing what
// matters for troubleshooting.
package processes

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/process"
)

func init() { modules.Register("processes", func() modules.Module { return &Module{} }) }

// Proc is one process entry.
type Proc struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Username   string  `json:"username,omitempty"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
	MemPercent float32 `json:"mem_percent"`
	Exe        string  `json:"exe,omitempty"`
	Priority   int32   `json:"priority"`
	Threads    int32   `json:"threads,omitempty"`
}

// Stats is the processes payload.
type Stats struct {
	Total  int    `json:"total"`   // total processes on the host
	SortBy string `json:"sort_by"` // cpu | mem
	Top    []Proc `json:"top"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
	topN   int
	sortBy string
}

func (m *Module) Name() string { return "processes" }

// Configure reads optional settings: top_n (default 15) and sort_by
// ("cpu" default, or "mem").
func (m *Module) Configure(settings map[string]any, log *slog.Logger) error {
	m.BaseModule.Configure(settings, log)
	m.topN = 15
	m.sortBy = "cpu"
	if v, ok := settings["top_n"]; ok {
		if n, ok := toInt(v); ok && n > 0 {
			m.topN = n
		}
	}
	if v, ok := settings["sort_by"].(string); ok && (v == "cpu" || v == "mem") {
		m.sortBy = v
	}
	return nil
}

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]Proc, 0, len(procs))
	for _, p := range procs {
		if ctx.Err() != nil {
			break
		}
		entry := Proc{PID: p.Pid}
		entry.Name, _ = p.NameWithContext(ctx)
		entry.Username, _ = p.UsernameWithContext(ctx)
		if cpu, err := p.CPUPercentWithContext(ctx); err == nil {
			entry.CPUPercent = round2(cpu)
		}
		if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
			entry.RSSBytes = mi.RSS
		}
		if mp, err := p.MemoryPercentWithContext(ctx); err == nil {
			entry.MemPercent = mp
		}
		entry.Exe, _ = p.ExeWithContext(ctx)
		if nice, err := p.NiceWithContext(ctx); err == nil {
			entry.Priority = nice
		}
		if nt, err := p.NumThreadsWithContext(ctx); err == nil {
			entry.Threads = nt
		}
		list = append(list, entry)
	}

	sort.Slice(list, func(i, j int) bool {
		if m.sortBy == "mem" {
			return list[i].RSSBytes > list[j].RSSBytes
		}
		return list[i].CPUPercent > list[j].CPUPercent
	})

	total := len(list)
	if len(list) > m.topN {
		list = list[:m.topN]
	}

	return json.Marshal(Stats{Total: total, SortBy: m.sortBy, Top: list})
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }

// toInt coerces a YAML-decoded numeric (int or float64) to int.
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
