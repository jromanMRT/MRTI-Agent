package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// newTestAgent builds an agent whose config lives in a temp dir, so
// applyConfig can persist pushed configs without touching the real host.
func newTestAgent(t *testing.T, mutate func(*config.Config)) (*Agent, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Agent.ID = "test-agent"
	cfg.Server.URL = "http://127.0.0.1:1/agent-core"
	cfg.Server.APIKey = "local-key"
	cfg.Cache.Path = filepath.Join(dir, "cache.db")
	cfg.Logging.Dir = filepath.Join(dir, "logs")
	cfg.Plugins.Dir = filepath.Join(dir, "plugins")
	if mutate != nil {
		mutate(cfg)
	}
	path := filepath.Join(dir, "config.yaml")
	cfg.SetPath(path)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.cache.Close() })
	return a, path
}

func pushConfig(t *testing.T, a *Agent, yaml string) model.CommandResult {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"yaml": yaml})
	res := model.CommandResult{}
	a.applyConfig(model.Command{ID: "cmd-test", Type: "set_config", Payload: payload}, &res)
	return res
}

const hostilePush = `
server:
  url: http://attacker.invalid
  api_key: stolen
scripts:
  enabled: true
  allowed_interpreters: [powershell]
update:
  enabled: true
  allow_unsigned: true
plugins:
  dir: C:\Windows\System32
remote_config:
  allow_security_changes: true
modules:
  enabled: [cpu, disk]
`

func TestSetConfigCannotEnableScriptsOrRepointServerRemotely(t *testing.T) {
	a, path := newTestAgent(t, nil)

	res := pushConfig(t, a, hostilePush)

	if !res.OK {
		t.Fatalf("non-protected changes should still apply, got error %q", res.Error)
	}
	for _, section := range []string{"remote_config", "server", "scripts", "update", "plugins.dir"} {
		if !strings.Contains(res.Output, section) {
			t.Errorf("result should report kept section %q: %s", section, res.Output)
		}
	}
	if a.cfg.Scripts.Enabled || a.cfg.Update.Enabled || a.cfg.Update.AllowUnsigned || a.cfg.RemoteConfig.AllowSecurityChanges {
		t.Fatalf("live config accepted a protected change: %+v", a.cfg)
	}
	if a.cfg.Server.URL != "http://127.0.0.1:1/agent-core" || a.cfg.Server.APIKey != "local-key" {
		t.Fatalf("server was repointed remotely: %+v", a.cfg.Server)
	}
	if a.pendingRestart {
		t.Fatal("a kept server section must not trigger a restart")
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Scripts.Enabled || saved.Update.AllowUnsigned || saved.Server.URL != "http://127.0.0.1:1/agent-core" || saved.RemoteConfig.AllowSecurityChanges {
		t.Fatalf("persisted config.yaml accepted a protected change: %+v", saved)
	}
	if strings.Join(saved.Modules.Enabled, ",") != "cpu,disk" {
		t.Fatalf("module change should be persisted, got %v", saved.Modules.Enabled)
	}
}

func TestSetConfigAppliesSecurityChangesWhenHostOptsIn(t *testing.T) {
	a, _ := newTestAgent(t, func(c *config.Config) { c.RemoteConfig.AllowSecurityChanges = true })

	res := pushConfig(t, a, hostilePush)

	if !res.OK || !a.cfg.Scripts.Enabled || a.cfg.Server.URL != "http://attacker.invalid" {
		t.Fatalf("opted-in host should accept security changes: ok=%v err=%q cfg=%+v", res.OK, res.Error, a.cfg)
	}
	if !a.cfg.RemoteConfig.AllowSecurityChanges {
		t.Fatal("policy must stay as configured locally")
	}
	if !a.pendingRestart {
		t.Fatal("a server change should request a restart")
	}
}

func TestSetConfigWithoutProtectedChangesHasCleanOutput(t *testing.T) {
	a, _ := newTestAgent(t, nil)
	// Mismas secciones protegidas que el equipo: solo cambian los módulos.
	same := "server:\n  url: http://127.0.0.1:1/agent-core\n  api_key: local-key\n" +
		"plugins:\n  dir: " + a.cfg.Plugins.Dir + "\nlogging:\n  dir: " + a.cfg.Logging.Dir +
		"\ncache:\n  path: " + a.cfg.Cache.Path + "\nmodules:\n  enabled: [cpu]\n"
	res := pushConfig(t, a, same)
	if !res.OK || strings.Contains(res.Output, "kept local values") {
		t.Fatalf("unexpected result: ok=%v out=%q err=%q", res.OK, res.Output, res.Error)
	}
}
