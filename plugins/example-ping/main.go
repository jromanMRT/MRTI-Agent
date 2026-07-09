// Command example-ping is a reference MRTI collector plugin. It measures TCP
// reachability/latency to a configurable target and returns the result as
// JSON. Build it as a standalone binary and drop it in the agent's plugins
// directory; no recompilation of the agent is required.
//
// Build: go build -o ../../plugins/example-ping/example-ping ./plugins/example-ping
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/pluginhost/shared"
)

// settings mirrors the module's config block.
type settings struct {
	Target  string `json:"target"`  // host:port, default 8.8.8.8:53
	Timeout string `json:"timeout"` // Go duration, default 2s
}

// result is the JSON payload this plugin emits.
type result struct {
	Target    string  `json:"target"`
	Reachable bool    `json:"reachable"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

type pinger struct {
	target  string
	timeout time.Duration
}

func (p *pinger) Info() (string, string, string) {
	return "ping", "1.0.0", "TCP reachability/latency probe (reference plugin)"
}

func (p *pinger) Configure(settingsJSON []byte) error {
	p.target = "8.8.8.8:53"
	p.timeout = 2 * time.Second
	if len(settingsJSON) == 0 {
		return nil
	}
	var s settings
	if err := json.Unmarshal(settingsJSON, &s); err != nil {
		return err
	}
	if s.Target != "" {
		p.target = s.Target
	}
	if s.Timeout != "" {
		if d, err := time.ParseDuration(s.Timeout); err == nil {
			p.timeout = d
		}
	}
	return nil
}

func (p *pinger) Collect() ([]byte, error) {
	if p.target == "" {
		p.target = "8.8.8.8:53"
		p.timeout = 2 * time.Second
	}
	res := result{Target: p.target}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", p.target, p.timeout)
	if err != nil {
		res.Error = err.Error()
		return json.Marshal(res)
	}
	conn.Close()
	res.Reachable = true
	res.LatencyMS = float64(time.Since(start).Microseconds()) / 1000.0
	return json.Marshal(res)
}

func main() {
	_ = fmt.Sprint // keep fmt import for future diagnostics
	shared.Serve(&pinger{})
}
