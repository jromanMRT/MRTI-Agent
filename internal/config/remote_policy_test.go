package config

import (
	"reflect"
	"testing"
)

func baseConfig() *Config {
	c := Default()
	c.Server.URL = "http://192.168.1.203/agent-core"
	c.Server.APIKey = "local-key"
	return c
}

func TestProtectFromRemoteKeepsSecuritySectionsByDefault(t *testing.T) {
	local := baseConfig()
	pushed := baseConfig()
	pushed.Server.URL = "http://attacker.invalid"
	pushed.Scripts.Enabled = true
	pushed.Scripts.AllowedInterpreters = []string{"powershell"}
	pushed.Update.Enabled = true
	pushed.Update.AllowUnsigned = true
	pushed.Plugins.Dir = `C:\Windows\System32`
	pushed.Logging.Dir = `C:\Windows`
	pushed.Cache.Path = `C:\Windows\x.db`
	pushed.RemoteConfig.AllowSecurityChanges = true
	pushed.Modules.Enabled = []string{"cpu", "disk"} // no protegido: sí se aplica

	kept := local.ProtectFromRemote(pushed)

	want := []string{"remote_config", "server", "scripts", "update", "plugins.dir", "logging.dir", "cache.path"}
	if !reflect.DeepEqual(kept, want) {
		t.Fatalf("kept = %v, want %v", kept, want)
	}
	if !reflect.DeepEqual(pushed.Server, local.Server) || !reflect.DeepEqual(pushed.Scripts, local.Scripts) ||
		!reflect.DeepEqual(pushed.Update, local.Update) || pushed.Plugins.Dir != local.Plugins.Dir ||
		pushed.Logging.Dir != local.Logging.Dir || pushed.Cache.Path != local.Cache.Path {
		t.Fatalf("protected sections were not restored: %+v", pushed)
	}
	if pushed.RemoteConfig.AllowSecurityChanges {
		t.Fatal("the policy itself must never be enabled remotely")
	}
	if !reflect.DeepEqual(pushed.Modules.Enabled, []string{"cpu", "disk"}) {
		t.Fatalf("non-protected change was dropped: %v", pushed.Modules.Enabled)
	}
}

func TestProtectFromRemoteNoChangesReportsNothing(t *testing.T) {
	if kept := baseConfig().ProtectFromRemote(baseConfig()); len(kept) != 0 {
		t.Fatalf("kept = %v, want none", kept)
	}
}

func TestProtectFromRemoteHonoursLocalOptIn(t *testing.T) {
	local := baseConfig()
	local.RemoteConfig.AllowSecurityChanges = true
	pushed := baseConfig()
	pushed.Scripts.Enabled = true
	pushed.Server.URL = "http://nuevo.local"
	pushed.RemoteConfig.AllowSecurityChanges = false // tampoco se puede apagar a distancia

	kept := local.ProtectFromRemote(pushed)

	if !reflect.DeepEqual(kept, []string{"remote_config"}) {
		t.Fatalf("kept = %v, want [remote_config]", kept)
	}
	if !pushed.Scripts.Enabled || pushed.Server.URL != "http://nuevo.local" || !pushed.RemoteConfig.AllowSecurityChanges {
		t.Fatalf("opted-in host should accept security changes but keep its policy: %+v", pushed)
	}
}

func TestDefaultPolicyIsLocked(t *testing.T) {
	cfg, err := FromBytes([]byte("server:\n  url: http://x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteConfig.AllowSecurityChanges {
		t.Fatal("remote security changes must be off unless the host opts in")
	}
}
