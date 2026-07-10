// Package model defines the data structures exchanged between the MRTI Agent
// and the MRTI Core server. Everything sent over the wire lives here so that
// modules, transport and cache all agree on a single schema.
package model

import (
	"encoding/json"
	"time"
)

// ModuleResult is what a single collection module (native or plugin) returns
// for one collection cycle. Data holds the module-specific payload which is
// serialized to JSON before being placed in an Envelope.
type ModuleResult struct {
	Module      string          `json:"module"`
	CollectedAt time.Time       `json:"collected_at"`
	Data        json.RawMessage `json:"data,omitempty"`
	Error       string          `json:"error,omitempty"`
	DurationMS  int64           `json:"duration_ms"`
}

// Envelope is the top-level message the agent ships to the server. One
// envelope aggregates the results of every enabled module for a cycle plus
// identity metadata so the Core can route it to the right host record.
type Envelope struct {
	Schema    string         `json:"schema"`   // schema version, e.g. "mrti.v1"
	AgentID   string         `json:"agent_id"` // stable per-install UUID
	Hostname  string         `json:"hostname"`
	Version   string         `json:"version"` // agent binary version
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Tags      []string       `json:"tags,omitempty"`
	Sequence  uint64         `json:"sequence"`
	Timestamp time.Time      `json:"timestamp"`
	Results   []ModuleResult `json:"results"`
}

// Heartbeat is a lightweight liveness ping distinct from full metric
// envelopes. It lets the Core mark an agent online cheaply and carries the
// currently active modules and self-reported resource usage.
type Heartbeat struct {
	Schema        string    `json:"schema"`
	AgentID       string    `json:"agent_id"`
	Hostname      string    `json:"hostname"`
	Version       string    `json:"version"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	Uptime        int64     `json:"uptime_seconds"`
	ActiveModules []string  `json:"active_modules"`
	SelfCPU       float64   `json:"self_cpu_percent"`
	SelfMemMB     float64   `json:"self_mem_mb"`
	Timestamp     time.Time `json:"timestamp"`
}

// Command is an instruction pushed from the Core to the agent (e.g. enable a
// module, run a remote script, trigger self-update). The agent polls or
// receives these over the transport channel.
type Command struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // ping | run_script | update | enable_module | disable_module | set_config
	Payload  json.RawMessage `json:"payload,omitempty"`
	IssuedAt time.Time       `json:"issued_at"`
}

// CommandResult is returned to the Core after a Command is processed.
type CommandResult struct {
	CommandID  string    `json:"command_id"`
	AgentID    string    `json:"agent_id"`
	OK         bool      `json:"ok"`
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

// SchemaVersion is the current wire schema identifier. Bump when the Envelope
// shape changes in a backward-incompatible way.
const SchemaVersion = "mrti.v1"
