// Package docker reports the local Docker containers: names, image, state,
// restart count and (optionally) live CPU/RAM. It talks to the Docker Engine
// API directly over the local socket/named-pipe with a small http.Client,
// avoiding the heavyweight official SDK to keep the agent light.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("docker", func() modules.Module { return &Module{} }) }

// Container is one container entry, normalised for the Core.
type Container struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Image        string  `json:"image"`
	State        string  `json:"state"`  // running | exited | paused | ...
	Status       string  `json:"status"` // human string, e.g. "Up 2 hours"
	RestartCount int     `json:"restart_count"`
	CPUPercent   float64 `json:"cpu_percent,omitempty"`
	MemUsageMB   float64 `json:"mem_usage_mb,omitempty"`
	MemLimitMB   float64 `json:"mem_limit_mb,omitempty"`
}

// Stats is the docker payload. Available is false when no Docker daemon is
// reachable, so the Core can distinguish "no docker" from "no containers".
type Stats struct {
	Available  bool        `json:"available"`
	Running    int         `json:"running"`
	Total      int         `json:"total"`
	Containers []Container `json:"containers,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
	host   string
	stats  bool
	client *http.Client
}

func (m *Module) Name() string { return "docker" }

// Configure reads optional settings: host (socket path / pipe, default
// per-OS) and stats (fetch live CPU/RAM, default false — it costs ~1s/container).
func (m *Module) Configure(settings map[string]any, log *slog.Logger) error {
	m.BaseModule.Configure(settings, log)
	m.host = defaultDockerHost()
	if v, ok := settings["host"].(string); ok && v != "" {
		m.host = v
	}
	if v, ok := settings["stats"].(bool); ok {
		m.stats = v
	}
	m.client = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialDocker(ctx, m.host)
			},
		},
	}
	return nil
}

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	list, err := m.listContainers(ctx)
	if err != nil {
		// No reachable daemon is a normal state, not a module failure.
		return json.Marshal(Stats{Available: false, Error: err.Error()})
	}

	out := Stats{Available: true, Total: len(list)}
	for i := range list {
		c := &list[i]
		if c.State == "running" {
			out.Running++
		}
		if rc, started, err := m.inspect(ctx, c.ID); err == nil {
			c.RestartCount = rc
			_ = started
		}
		if m.stats && c.State == "running" {
			m.fillStats(ctx, c)
		}
	}
	out.Containers = list
	return json.Marshal(out)
}

// listContainers calls GET /containers/json?all=1.
func (m *Module) listContainers(ctx context.Context) ([]Container, error) {
	var raw []struct {
		ID     string   `json:"Id"`
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := m.get(ctx, "/containers/json?all=1", &raw); err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(raw))
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		id := r.ID
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, Container{
			ID: id, Name: name, Image: r.Image, State: r.State, Status: r.Status,
		})
	}
	return out, nil
}

// inspect returns the restart count and start time for a container.
func (m *Module) inspect(ctx context.Context, id string) (int, string, error) {
	var info struct {
		RestartCount int `json:"RestartCount"`
		State        struct {
			StartedAt string `json:"StartedAt"`
		} `json:"State"`
	}
	if err := m.get(ctx, "/containers/"+id+"/json", &info); err != nil {
		return 0, "", err
	}
	return info.RestartCount, info.State.StartedAt, nil
}

// fillStats fetches a one-shot resource sample for a running container.
func (m *Module) fillStats(ctx context.Context, c *Container) {
	var s struct {
		CPUStats    cpuStats `json:"cpu_stats"`
		PreCPUStats cpuStats `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
			Stats struct {
				Cache uint64 `json:"cache"`
			} `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := m.get(ctx, "/containers/"+c.ID+"/stats?stream=false", &s); err != nil {
		return
	}
	c.CPUPercent = round2(calcCPUPercent(s.PreCPUStats, s.CPUStats))
	usage := s.MemoryStats.Usage
	if usage > s.MemoryStats.Stats.Cache {
		usage -= s.MemoryStats.Stats.Cache
	}
	c.MemUsageMB = round2(float64(usage) / (1024 * 1024))
	c.MemLimitMB = round2(float64(s.MemoryStats.Limit) / (1024 * 1024))
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage  uint64   `json:"total_usage"`
		PercpuUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint32 `json:"online_cpus"`
}

func calcCPUPercent(pre, cur cpuStats) float64 {
	cpuDelta := float64(cur.CPUUsage.TotalUsage) - float64(pre.CPUUsage.TotalUsage)
	sysDelta := float64(cur.SystemUsage) - float64(pre.SystemUsage)
	cpus := float64(cur.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(cur.CPUUsage.PercpuUsage))
	}
	if sysDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / sysDelta) * cpus * 100.0
	}
	return 0
}

// get performs a GET against the Docker API and decodes JSON into v.
func (m *Module) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("docker api %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
