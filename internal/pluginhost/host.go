// Package pluginhost loads external gRPC collector plugins and adapts each one
// to the modules.Module interface, so the agent orchestrator treats plugins
// exactly like built-in modules. Each plugin runs as its own child process
// (via HashiCorp go-plugin); if one crashes it is isolated from the agent and
// the other plugins.
package pluginhost

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/jromanMRT/mrti-agent/internal/pluginhost/shared"
	"github.com/jromanMRT/mrti-agent/modules"
)

// pluginModule adapts a running plugin to modules.Module.
type pluginModule struct {
	name   string
	client shared.PluginClient
	proc   *goplugin.Client // owns the child process; killed on Close
	log    *slog.Logger
}

func (p *pluginModule) Name() string { return p.name }

func (p *pluginModule) Configure(settings map[string]any, log *slog.Logger) error {
	p.log = log
	var raw []byte
	if len(settings) > 0 {
		b, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		raw = b
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10e9)
	defer cancel()
	return p.client.Configure(ctx, raw)
}

func (p *pluginModule) Collect(ctx context.Context) (json.RawMessage, error) {
	data, err := p.client.Collect(ctx)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (p *pluginModule) Close() error {
	p.proc.Kill()
	return nil
}

// Load launches each enabled plugin found under dir and returns the resulting
// modules. A plugin that fails to start is logged and skipped rather than
// aborting the whole agent. The returned modules must be Closed by the caller.
func Load(dir string, enabled []string, log *slog.Logger) []modules.Module {
	var loaded []modules.Module
	for _, name := range enabled {
		path, err := resolveBinary(dir, name)
		if err != nil {
			log.Warn("plugin not found", "plugin", name, "err", err)
			continue
		}
		mod, err := launch(name, path, log)
		if err != nil {
			log.Error("plugin failed to start", "plugin", name, "err", err)
			continue
		}
		log.Info("plugin loaded", "plugin", name, "path", path)
		loaded = append(loaded, mod)
	}
	return loaded
}

// resolveBinary locates a plugin executable. It accepts either
// <dir>/<name>[.exe] or <dir>/<name>/<name>[.exe].
func resolveBinary(dir, name string) (string, error) {
	exe := name
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	candidates := []string{
		filepath.Join(dir, exe),
		filepath.Join(dir, name, exe),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("no executable for plugin %q under %s", name, dir)
}

func launch(name, path string, log *slog.Logger) (modules.Module, error) {
	hlog := hclog.New(&hclog.LoggerOptions{
		Name:   "mrti-plugin." + name,
		Level:  hclog.Warn,
		Output: os.Stderr,
	})

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  shared.Handshake,
		Plugins:          shared.PluginMap(nil), // client side needs no impl
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           hlog,
		Managed:          false,
	})

	rpc, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, err
	}
	raw, err := rpc.Dispense(shared.PluginName)
	if err != nil {
		client.Kill()
		return nil, err
	}
	pc, ok := raw.(shared.PluginClient)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin %q did not implement PluginClient", name)
	}

	return &pluginModule{name: name, client: pc, proc: client, log: log}, nil
}
