// Package ram collects physical and swap memory statistics.
package ram

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/mem"
)

func init() { modules.Register("ram", func() modules.Module { return &Module{} }) }

// Stats is the memory payload. Byte counts are reported raw so the Core can
// format them; percentages are convenience values.
type Stats struct {
	Total       uint64  `json:"total_bytes"`
	Available   uint64  `json:"available_bytes"`
	Used        uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Cached      uint64  `json:"cached_bytes,omitempty"`
	SwapTotal   uint64  `json:"swap_total_bytes"`
	SwapUsed    uint64  `json:"swap_used_bytes"`
	SwapPercent float64 `json:"swap_used_percent"`
}

type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "ram" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	s := Stats{
		Total:       v.Total,
		Available:   v.Available,
		Used:        v.Used,
		UsedPercent: round2(v.UsedPercent),
		Cached:      v.Cached,
	}
	if sw, err := mem.SwapMemoryWithContext(ctx); err == nil && sw != nil {
		s.SwapTotal = sw.Total
		s.SwapUsed = sw.Used
		s.SwapPercent = round2(sw.UsedPercent)
	}
	return json.Marshal(s)
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
