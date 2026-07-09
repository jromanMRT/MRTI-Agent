// Package disk collects filesystem usage per mounted partition and per-device
// I/O counters. SMART health is gathered best-effort via smartctl when the
// smartmontools package is present (see smart.go).
package disk

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/disk"
)

func init() { modules.Register("disk", func() modules.Module { return &Module{} }) }

// Partition describes one mounted filesystem.
type Partition struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total_bytes"`
	Used        uint64  `json:"used_bytes"`
	Free        uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// IOStat describes cumulative I/O counters for one physical device.
type IOStat struct {
	Name       string `json:"name"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadCount  uint64 `json:"read_count"`
	WriteCount uint64 `json:"write_count"`
	IOTimeMS   uint64 `json:"io_time_ms"`
}

// Stats is the disk payload.
type Stats struct {
	Partitions []Partition `json:"partitions"`
	IO         []IOStat    `json:"io,omitempty"`
	SMART      []SMARTInfo `json:"smart,omitempty"`
}

type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "disk" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	var s Stats

	parts, err := disk.PartitionsWithContext(ctx, false) // physical only
	if err != nil {
		return nil, err
	}
	for _, p := range parts {
		u, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || u == nil {
			continue
		}
		s.Partitions = append(s.Partitions, Partition{
			Device:      p.Device,
			Mountpoint:  p.Mountpoint,
			Fstype:      p.Fstype,
			Total:       u.Total,
			Used:        u.Used,
			Free:        u.Free,
			UsedPercent: round2(u.UsedPercent),
		})
	}

	if io, err := disk.IOCountersWithContext(ctx); err == nil {
		for name, c := range io {
			s.IO = append(s.IO, IOStat{
				Name:       name,
				ReadBytes:  c.ReadBytes,
				WriteBytes: c.WriteBytes,
				ReadCount:  c.ReadCount,
				WriteCount: c.WriteCount,
				IOTimeMS:   c.IoTime,
			})
		}
	}

	// SMART is optional and can be slow; guard it behind availability.
	s.SMART = collectSMART(ctx, m.Log)

	return json.Marshal(s)
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
