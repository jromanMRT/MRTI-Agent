// Package agent is the orchestrator that ties everything together: it builds
// the active module set (native modules + external plugins), runs the timed
// collection / heartbeat / flush / command loops, buffers everything through
// the durable cache and ships it via the configured transport. This is the
// heart of the MRTI Agent runtime.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/jromanMRT/mrti-agent/internal/alerts"
	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/cache"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
	"github.com/jromanMRT/mrti-agent/internal/pluginhost"
	"github.com/jromanMRT/mrti-agent/internal/scripts"
	"github.com/jromanMRT/mrti-agent/internal/transport"
	"github.com/jromanMRT/mrti-agent/internal/updater"
	"github.com/jromanMRT/mrti-agent/modules"

	// Blank imports register the built-in modules with the module registry.
	_ "github.com/jromanMRT/mrti-agent/modules/cpu"
	_ "github.com/jromanMRT/mrti-agent/modules/disk"
	_ "github.com/jromanMRT/mrti-agent/modules/docker"
	_ "github.com/jromanMRT/mrti-agent/modules/eventlogs"
	_ "github.com/jromanMRT/mrti-agent/modules/inventory"
	_ "github.com/jromanMRT/mrti-agent/modules/network"
	_ "github.com/jromanMRT/mrti-agent/modules/processes"
	_ "github.com/jromanMRT/mrti-agent/modules/ram"
	_ "github.com/jromanMRT/mrti-agent/modules/services"
	_ "github.com/jromanMRT/mrti-agent/modules/snmp"
	_ "github.com/jromanMRT/mrti-agent/modules/software"
	_ "github.com/jromanMRT/mrti-agent/modules/system"
	_ "github.com/jromanMRT/mrti-agent/modules/temperature"
	_ "github.com/jromanMRT/mrti-agent/modules/ups"
	_ "github.com/jromanMRT/mrti-agent/modules/virtualization"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/process"
)

// Version is the agent build version, overridable via -ldflags.
var Version = "0.1.0-dev"

// Agent is the running instance.
type Agent struct {
	cfg   *config.Config
	log   *slog.Logger
	cache *cache.Cache
	tr    transport.Transport
	auth  *auth.Authenticator

	mu   sync.RWMutex
	mods []modules.Module

	seq       uint64
	startTime time.Time
	self      *process.Process
	scripts   *scripts.Executor
	updater   *updater.Updater

	pendingRestart bool // set when a self-update was applied; triggers process exit

	hostname string
}

// New constructs an Agent: ensures identity, opens the cache, builds the
// transport and instantiates every enabled module and plugin.
func New(cfg *config.Config, log *slog.Logger) (*Agent, error) {
	if err := ensureAgentID(cfg); err != nil {
		return nil, err
	}

	c, err := cache.Open(cfg.Cache.Path, cfg.Cache.MaxQueue)
	if err != nil {
		return nil, fmt.Errorf("open cache: %w", err)
	}

	authn := auth.New(cfg.Server)
	tr, err := transport.New(cfg, authn)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("init transport: %w", err)
	}

	hostname, _ := os.Hostname()
	self, _ := process.NewProcess(int32(os.Getpid()))

	a := &Agent{
		cfg:       cfg,
		log:       log,
		cache:     c,
		tr:        tr,
		auth:      authn,
		startTime: time.Now(),
		self:      self,
		scripts:   scripts.NewExecutor(cfg.Scripts),
		updater:   updater.New(cfg.Update, nil),
		hostname:  hostname,
	}

	a.buildModules()
	return a, nil
}

// buildModules instantiates native modules from the enabled list plus any
// gRPC plugins, and configures each one.
func (a *Agent) buildModules() {
	var active []modules.Module

	for _, name := range a.cfg.Modules.Enabled {
		m, err := modules.New(name)
		if err != nil {
			a.log.Warn("skipping unknown module", "module", name, "err", err)
			continue
		}
		if err := m.Configure(a.cfg.ModuleSettings(name), a.log.With("module", name)); err != nil {
			a.log.Warn("module configure failed", "module", name, "err", err)
			continue
		}
		active = append(active, m)
	}

	// External gRPC plugins are appended and treated identically.
	plugins := pluginhost.Load(a.cfg.Plugins.Dir, a.cfg.Plugins.Enabled, a.log)
	for _, p := range plugins {
		if err := p.Configure(a.cfg.ModuleSettings(p.Name()), a.log.With("plugin", p.Name())); err != nil {
			a.log.Warn("plugin configure failed", "plugin", p.Name(), "err", err)
			p.Close()
			continue
		}
		active = append(active, p)
	}

	a.mu.Lock()
	a.mods = active
	a.mu.Unlock()

	a.log.Info("modules active", "count", len(active), "names", a.moduleNames())
}

func (a *Agent) moduleNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	names := make([]string, 0, len(a.mods))
	for _, m := range a.mods {
		names = append(names, m.Name())
	}
	return names
}

// Run starts all periodic loops and blocks until ctx is cancelled, then shuts
// down cleanly.
func (a *Agent) Run(ctx context.Context) error {
	a.log.Info("mrti-agent starting",
		"version", Version, "agent_id", a.cfg.Agent.ID,
		"transport", a.cfg.Server.Transport, "server", a.cfg.Server.URL)

	var wg sync.WaitGroup
	loop := func(every time.Duration, fn func(context.Context)) {
		if every <= 0 {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Fire once promptly so the Core sees the agent immediately.
			fn(ctx)
			t := time.NewTicker(every)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					fn(ctx)
				}
			}
		}()
	}

	loop(a.cfg.Intervals.Collect, a.collectOnce)
	loop(a.cfg.Intervals.Heartbeat, a.heartbeatOnce)
	loop(a.cfg.Intervals.Flush, a.flushOnce)
	loop(a.cfg.Intervals.Commands, a.pollOnce)

	<-ctx.Done()
	a.log.Info("shutdown requested, draining")
	wg.Wait()
	a.shutdown()
	return nil
}

// collectOnce runs every module, assembles an Envelope and persists it.
func (a *Agent) collectOnce(ctx context.Context) {
	a.mu.RLock()
	mods := append([]modules.Module(nil), a.mods...)
	a.mu.RUnlock()

	results := make([]model.ModuleResult, 0, len(mods))
	for _, m := range mods {
		mctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		start := time.Now()
		data, err := m.Collect(mctx)
		cancel()

		res := model.ModuleResult{
			Module:      m.Name(),
			CollectedAt: start,
			Data:        data,
			DurationMS:  time.Since(start).Milliseconds(),
		}
		if err != nil {
			res.Error = err.Error()
			a.log.Warn("module collect error", "module", m.Name(), "err", err)
		}
		results = append(results, res)
	}

	// Evaluate local threshold alerts and attach them as a synthetic module
	// result so they travel with the metrics that triggered them.
	if fired := alerts.Evaluate(a.cfg.Alerts, results); len(fired) > 0 {
		if data, err := json.Marshal(fired); err == nil {
			results = append(results, model.ModuleResult{
				Module:      "alerts",
				CollectedAt: time.Now(),
				Data:        data,
			})
		}
		for _, al := range fired {
			a.log.Warn("alert", "rule", al.Rule, "severity", al.Severity,
				"resource", al.Resource, "value", al.Value)
		}
	}

	a.seq++
	env := model.Envelope{
		Schema:    model.SchemaVersion,
		AgentID:   a.cfg.Agent.ID,
		Hostname:  a.hostname,
		Version:   Version,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Tags:      a.cfg.Agent.Tags,
		Sequence:  a.seq,
		Timestamp: time.Now(),
		Results:   results,
	}
	payload, err := json.Marshal(env)
	if err != nil {
		a.log.Error("marshal envelope", "err", err)
		return
	}
	if err := a.cache.Enqueue("envelope", payload); err != nil {
		a.log.Error("cache enqueue", "err", err)
		return
	}
	a.log.Debug("envelope queued", "seq", a.seq, "modules", len(results))
}

// heartbeatOnce sends a lightweight liveness ping directly (best effort; not
// queued, since a stale heartbeat has no value).
func (a *Agent) heartbeatOnce(ctx context.Context) {
	hb := model.Heartbeat{
		Schema:        model.SchemaVersion,
		AgentID:       a.cfg.Agent.ID,
		Hostname:      a.hostname,
		Version:       Version,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ActiveModules: a.moduleNames(),
		Timestamp:     time.Now(),
	}
	if up, err := host.Uptime(); err == nil {
		hb.Uptime = int64(up)
	}
	a.fillSelfMetrics(&hb)

	payload, err := json.Marshal(hb)
	if err != nil {
		return
	}
	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.tr.Send(sctx, "heartbeat", payload); err != nil {
		a.log.Debug("heartbeat send failed", "err", err)
	}
}

// fillSelfMetrics records the agent's own CPU/memory footprint so operators
// can prove the agent is light.
func (a *Agent) fillSelfMetrics(hb *model.Heartbeat) {
	if a.self == nil {
		return
	}
	if pct, err := a.self.CPUPercent(); err == nil {
		hb.SelfCPU = round2(pct)
	}
	if mi, err := a.self.MemoryInfo(); err == nil && mi != nil {
		hb.SelfMemMB = round2(float64(mi.RSS) / (1024 * 1024))
	}
}

// flushOnce drains the outbox in order, stopping at the first send failure so
// ordering and at-least-once delivery are preserved.
func (a *Agent) flushOnce(ctx context.Context) {
	items, err := a.cache.Peek(50)
	if err != nil {
		a.log.Error("cache peek", "err", err)
		return
	}
	if len(items) == 0 {
		return
	}

	var acked []int64
	for _, it := range items {
		sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := a.tr.Send(sctx, it.Kind, it.Payload)
		cancel()
		if err != nil {
			a.log.Debug("flush send failed, will retry", "err", err, "queued", len(items)-len(acked))
			break
		}
		acked = append(acked, it.ID)
	}
	if len(acked) > 0 {
		if err := a.cache.Ack(acked); err != nil {
			a.log.Error("cache ack", "err", err)
		}
		a.log.Debug("flushed", "sent", len(acked))
	}
}

// pollOnce fetches and dispatches commands from the Core.
func (a *Agent) pollOnce(ctx context.Context) {
	pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmds, err := a.tr.Poll(pctx, a.cfg.Agent.ID)
	if err != nil {
		a.log.Debug("poll commands failed", "err", err)
		return
	}
	for _, cmd := range cmds {
		a.handleCommand(ctx, cmd)
	}
}

// handleCommand executes a Core-issued command and queues the result. The full
// command set (run_script, self-update, module toggling) lands here as those
// subsystems come online; unknown commands are acknowledged with an error.
func (a *Agent) handleCommand(ctx context.Context, cmd model.Command) {
	res := model.CommandResult{
		CommandID:  cmd.ID,
		AgentID:    a.cfg.Agent.ID,
		FinishedAt: time.Now(),
	}
	switch cmd.Type {
	case "ping", "noop":
		res.OK = true
		res.Output = "pong"
	case "run_script":
		a.runScript(ctx, cmd, &res)
	case "update":
		a.applyUpdate(ctx, cmd, &res)
	case "enable_module":
		a.toggleModule(cmd, &res, true)
	case "disable_module":
		a.toggleModule(cmd, &res, false)
	default:
		res.OK = false
		res.Error = fmt.Sprintf("command type %q not supported by this agent version", cmd.Type)
		a.log.Info("unsupported command", "type", cmd.Type, "id", cmd.ID)
	}
	res.FinishedAt = time.Now()
	if payload, err := json.Marshal(res); err == nil {
		_ = a.cache.Enqueue("command_result", payload)
	}

	// A self-update swapped the binary: report the result to the Core, then
	// exit so the service manager (systemd / Windows SC) relaunches the new
	// binary. In foreground mode this simply stops the agent.
	if a.pendingRestart {
		a.log.Info("self-update applied; flushing and restarting")
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		a.flushOnce(flushCtx)
		cancel()
		os.Exit(0)
	}
}

// runScript executes a Core-issued "run_script" command and records the full
// execution result (stdout/stderr/exit) as JSON in the command result output.
func (a *Agent) runScript(ctx context.Context, cmd model.Command, res *model.CommandResult) {
	var req scripts.Request
	if err := json.Unmarshal(cmd.Payload, &req); err != nil {
		res.Error = "invalid run_script payload: " + err.Error()
		return
	}
	a.log.Info("running remote script", "id", cmd.ID, "interpreter", req.Interpreter)
	result := a.scripts.Run(ctx, req)

	if out, err := json.Marshal(result); err == nil {
		res.Output = string(out)
	}
	res.OK = result.Error == "" && !result.TimedOut && result.ExitCode == 0
	if !res.OK && res.Error == "" {
		if result.Error != "" {
			res.Error = result.Error
		} else {
			res.Error = fmt.Sprintf("script exited with code %d", result.ExitCode)
		}
	}
}

func (a *Agent) applyUpdate(ctx context.Context, cmd model.Command, res *model.CommandResult) {
	var req updater.Request
	if err := json.Unmarshal(cmd.Payload, &req); err != nil {
		res.Error = "invalid update payload: " + err.Error()
		return
	}

	target, err := os.Executable()
	if err != nil {
		res.Error = fmt.Sprintf("resolve executable path: %v", err)
		return
	}

	a.log.Info("self-update requested", "id", cmd.ID, "target", target, "version", req.Version)
	result := a.updater.Apply(ctx, req, target)
	if out, err := json.Marshal(result); err == nil {
		res.Output = string(out)
	}
	res.OK = result.Applied
	if !res.OK && res.Error == "" {
		res.Error = result.Error
	}
	if result.Applied {
		// The new binary is in place; ask the run loop to exit so the service
		// manager relaunches it. Handled after the result is queued.
		a.pendingRestart = true
	}
}

func (a *Agent) toggleModule(cmd model.Command, res *model.CommandResult, enable bool) {
	var req struct {
		Module string `json:"module"`
	}
	if err := json.Unmarshal(cmd.Payload, &req); err != nil {
		res.Error = "invalid module command payload: " + err.Error()
		return
	}
	if req.Module == "" {
		res.Error = "module name is required"
		return
	}

	if err := a.setModuleEnabled(req.Module, enable); err != nil {
		res.Error = err.Error()
		return
	}

	a.log.Info("module toggle", "action", func() string {
		if enable {
			return "enable"
		}
		return "disable"
	}(), "module", req.Module)
	res.OK = true
	action := "enabled"
	if !enable {
		action = "disabled"
	}
	res.Output = fmt.Sprintf("module %q %s", req.Module, action)
}

func (a *Agent) setModuleEnabled(name string, enable bool) error {
	current := a.cfg.Modules.Enabled
	if enable {
		if containsString(current, name) {
			return nil
		}
		if _, err := modules.New(name); err != nil {
			return fmt.Errorf("unknown module %q", name)
		}
		current = append(current, name)
		sort.Strings(current)
	} else {
		if !containsString(current, name) {
			return nil
		}
		current = removeString(current, name)
	}
	a.cfg.Modules.Enabled = current
	a.buildModules()
	if a.cfg.Path() != "" {
		if err := a.cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}
	return nil
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func removeString(items []string, value string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == value {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (a *Agent) shutdown() {
	a.mu.RLock()
	mods := a.mods
	a.mu.RUnlock()
	for _, m := range mods {
		_ = m.Close()
	}
	if a.tr != nil {
		_ = a.tr.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	a.log.Info("mrti-agent stopped")
}

// ensureAgentID generates and persists a stable UUID on first run.
func ensureAgentID(cfg *config.Config) error {
	if cfg.Agent.ID != "" {
		return nil
	}
	cfg.Agent.ID = uuid.NewString()
	if cfg.Path() != "" {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("persist generated agent id: %w", err)
		}
	}
	return nil
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
