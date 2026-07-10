package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/model"
	_ "modernc.org/sqlite"
)

// Store is the Core's persistence layer: a small SQLite database holding the
// latest state per agent, per-module data, recent alerts and the command queue.
type Store struct {
	db *sql.DB
}

// AgentRow is the summary record for one agent.
type AgentRow struct {
	ID            string          `json:"id"`
	Hostname      string          `json:"hostname"`
	Name          string          `json:"name,omitempty"`
	OS            string          `json:"os"`
	Arch          string          `json:"arch"`
	Version       string          `json:"version"`
	Tags          json.RawMessage `json:"tags,omitempty"`
	FirstSeen     int64           `json:"first_seen"`
	LastSeen      int64           `json:"last_seen"`
	LastHeartbeat int64           `json:"last_heartbeat"`
	SelfCPU       float64         `json:"self_cpu_percent"`
	SelfMemMB     float64         `json:"self_mem_mb"`
	Sequence      uint64          `json:"sequence"`
	Online        bool            `json:"online"`
}

// AlertRow is one stored alert event.
type AlertRow struct {
	ID        int64   `json:"id"`
	AgentID   string  `json:"agent_id"`
	Hostname  string  `json:"hostname"`
	Rule      string  `json:"rule"`
	Severity  string  `json:"severity"`
	Resource  string  `json:"resource"`
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Timestamp int64   `json:"timestamp"`
}

func openStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, p := range []string{
		"PRAGMA journal_mode=WAL;", "PRAGMA synchronous=NORMAL;", "PRAGMA busy_timeout=5000;",
	} {
		if _, err := db.Exec(p); err != nil {
			return nil, err
		}
	}
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY, hostname TEXT, name TEXT, os TEXT, arch TEXT, version TEXT,
		tags TEXT, first_seen INTEGER, last_seen INTEGER, last_heartbeat INTEGER,
		self_cpu REAL, self_mem REAL, sequence INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS latest (
		agent_id TEXT PRIMARY KEY, sequence INTEGER, ts INTEGER, envelope BLOB
	);
	CREATE TABLE IF NOT EXISTS module_data (
		agent_id TEXT, module TEXT, ts INTEGER, data BLOB,
		PRIMARY KEY (agent_id, module)
	);
	CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT, hostname TEXT,
		rule TEXT, severity TEXT, resource TEXT, message TEXT, value REAL, threshold REAL, ts INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_alerts_ts ON alerts(ts DESC);
	CREATE TABLE IF NOT EXISTS commands (
		id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT, cmd_id TEXT, type TEXT,
		payload BLOB, created INTEGER, delivered INTEGER DEFAULT 0,
		result BLOB, result_ok INTEGER, result_ts INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_cmd_agent ON commands(agent_id, delivered);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// touchAgent upserts the agent identity/liveness fields.
func (s *Store) touchAgent(id, hostname, name, os, arch, version string, tags []byte, now int64) {
	s.db.Exec(`
		INSERT INTO agents (id, hostname, name, os, arch, version, tags, first_seen, last_seen)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			hostname=excluded.hostname, name=COALESCE(NULLIF(excluded.name,''), agents.name),
			os=excluded.os, arch=excluded.arch, version=excluded.version,
			tags=excluded.tags, last_seen=excluded.last_seen`,
		id, hostname, name, os, arch, version, string(tags), now, now)
}

// IngestHeartbeat updates liveness and self-usage from a heartbeat.
func (s *Store) IngestHeartbeat(hb model.Heartbeat) {
	now := time.Now().Unix()
	s.touchAgent(hb.AgentID, hb.Hostname, "", hb.OS, hb.Arch, hb.Version, nil, now)
	s.db.Exec(`UPDATE agents SET last_heartbeat=?, self_cpu=?, self_mem=? WHERE id=?`,
		now, hb.SelfCPU, hb.SelfMemMB, hb.AgentID)
}

// IngestEnvelope stores the full envelope, per-module latest data and any alerts.
func (s *Store) IngestEnvelope(env model.Envelope) {
	now := time.Now().Unix()
	tags, _ := json.Marshal(env.Tags)
	s.touchAgent(env.AgentID, env.Hostname, "", env.OS, env.Arch, env.Version, tags, now)
	s.db.Exec(`UPDATE agents SET sequence=? WHERE id=?`, env.Sequence, env.AgentID)

	raw, _ := json.Marshal(env)
	s.db.Exec(`INSERT INTO latest (agent_id, sequence, ts, envelope) VALUES (?,?,?,?)
		ON CONFLICT(agent_id) DO UPDATE SET sequence=excluded.sequence, ts=excluded.ts, envelope=excluded.envelope`,
		env.AgentID, env.Sequence, now, raw)

	for _, r := range env.Results {
		if len(r.Data) == 0 {
			continue
		}
		s.db.Exec(`INSERT INTO module_data (agent_id, module, ts, data) VALUES (?,?,?,?)
			ON CONFLICT(agent_id, module) DO UPDATE SET ts=excluded.ts, data=excluded.data`,
			env.AgentID, r.Module, r.CollectedAt.Unix(), []byte(r.Data))

		if r.Module == "alerts" {
			s.storeAlerts(env.AgentID, env.Hostname, r.Data)
		}
	}
}

func (s *Store) storeAlerts(agentID, hostname string, data json.RawMessage) {
	var alerts []struct {
		Rule      string  `json:"rule"`
		Severity  string  `json:"severity"`
		Resource  string  `json:"resource"`
		Message   string  `json:"message"`
		Value     float64 `json:"value"`
		Threshold float64 `json:"threshold"`
	}
	if json.Unmarshal(data, &alerts) != nil {
		return
	}
	now := time.Now().Unix()
	for _, a := range alerts {
		s.db.Exec(`INSERT INTO alerts (agent_id, hostname, rule, severity, resource, message, value, threshold, ts)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			agentID, hostname, a.Rule, a.Severity, a.Resource, a.Message, a.Value, a.Threshold, now)
	}
}

// SaveCommandResult records the outcome of a delivered command.
func (s *Store) SaveCommandResult(res model.CommandResult) {
	raw, _ := json.Marshal(res)
	ok := 0
	if res.OK {
		ok = 1
	}
	s.db.Exec(`UPDATE commands SET result=?, result_ok=?, result_ts=? WHERE cmd_id=? AND agent_id=?`,
		raw, ok, time.Now().Unix(), res.CommandID, res.AgentID)
}

// EnqueueCommand queues a command for an agent to pick up.
func (s *Store) EnqueueCommand(agentID string, cmd model.Command) {
	s.db.Exec(`INSERT INTO commands (agent_id, cmd_id, type, payload, created) VALUES (?,?,?,?,?)`,
		agentID, cmd.ID, cmd.Type, []byte(cmd.Payload), time.Now().Unix())
}

// TakeCommands returns and marks-delivered the pending commands for an agent.
func (s *Store) TakeCommands(agentID string) []model.Command {
	rows, err := s.db.Query(`SELECT id, cmd_id, type, payload FROM commands
		WHERE agent_id=? AND delivered=0 ORDER BY id ASC`, agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var cmds []model.Command
	var ids []int64
	for rows.Next() {
		var id int64
		var c model.Command
		var payload []byte
		rows.Scan(&id, &c.ID, &c.Type, &payload)
		c.Payload = payload
		cmds = append(cmds, c)
		ids = append(ids, id)
	}
	for _, id := range ids {
		s.db.Exec(`UPDATE commands SET delivered=1 WHERE id=?`, id)
	}
	return cmds
}

// onlineWindow: an agent is "online" if seen within this many seconds.
const onlineWindow = 120

// ListAgents returns all agents with computed online status.
func (s *Store) ListAgents() []AgentRow {
	rows, err := s.db.Query(`SELECT id, hostname, name, os, arch, version, tags,
		first_seen, last_seen, last_heartbeat, self_cpu, self_mem, sequence
		FROM agents ORDER BY hostname`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	now := time.Now().Unix()
	var out []AgentRow
	for rows.Next() {
		var a AgentRow
		var tags sql.NullString
		rows.Scan(&a.ID, &a.Hostname, &a.Name, &a.OS, &a.Arch, &a.Version, &tags,
			&a.FirstSeen, &a.LastSeen, &a.LastHeartbeat, &a.SelfCPU, &a.SelfMemMB, &a.Sequence)
		if tags.Valid && tags.String != "" && tags.String != "null" {
			a.Tags = json.RawMessage(tags.String)
		}
		a.Online = now-a.LastSeen <= onlineWindow
		out = append(out, a)
	}
	return out
}

// GetEnvelope returns the latest full envelope JSON for an agent.
func (s *Store) GetEnvelope(agentID string) (json.RawMessage, bool) {
	var raw []byte
	err := s.db.QueryRow(`SELECT envelope FROM latest WHERE agent_id=?`, agentID).Scan(&raw)
	if err != nil {
		return nil, false
	}
	return raw, true
}

// GetModule returns the latest data JSON for one module of an agent.
func (s *Store) GetModule(agentID, module string) (json.RawMessage, bool) {
	var raw []byte
	err := s.db.QueryRow(`SELECT data FROM module_data WHERE agent_id=? AND module=?`, agentID, module).Scan(&raw)
	if err != nil {
		return nil, false
	}
	return raw, true
}

// RecentAlerts returns the most recent alerts across the fleet.
func (s *Store) RecentAlerts(limit int) []AlertRow {
	rows, err := s.db.Query(`SELECT id, agent_id, hostname, rule, severity, resource, message, value, threshold, ts
		FROM alerts ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []AlertRow
	for rows.Next() {
		var a AlertRow
		rows.Scan(&a.ID, &a.AgentID, &a.Hostname, &a.Rule, &a.Severity, &a.Resource,
			&a.Message, &a.Value, &a.Threshold, &a.Timestamp)
		out = append(out, a)
	}
	return out
}

// modulesFor returns the latest per-module data map for an agent.
func (s *Store) modulesFor(agentID string) map[string]json.RawMessage {
	rows, err := s.db.Query(`SELECT module, data FROM module_data WHERE agent_id=?`, agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var m string
		var d []byte
		rows.Scan(&m, &d)
		out[m] = json.RawMessage(d)
	}
	return out
}

// ExportAll returns the full fleet state for the /export endpoint.
func (s *Store) ExportAll() map[string]any {
	agents := s.ListAgents()
	fleet := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		fleet = append(fleet, map[string]any{
			"agent":   a,
			"modules": s.modulesFor(a.ID),
		})
	}
	return map[string]any{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"agent_count": len(agents),
		"agents":      fleet,
		"alerts":      s.RecentAlerts(200),
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return b
}
