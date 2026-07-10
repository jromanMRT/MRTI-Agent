// Package config loads and validates the agent configuration from config.yaml,
// applying sensible defaults and environment-variable overrides. The config is
// intentionally the single source of truth for what the agent does; the MRTI
// Core can push updated config so behaviour is controllable centrally.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the full agent configuration tree.
type Config struct {
	Agent     AgentConfig     `yaml:"agent"`
	Server    ServerConfig    `yaml:"server"`
	Intervals IntervalsConfig `yaml:"intervals"`
	Logging   LoggingConfig   `yaml:"logging"`
	Cache     CacheConfig     `yaml:"cache"`
	Modules   ModulesConfig   `yaml:"modules"`
	Plugins   PluginsConfig   `yaml:"plugins"`
	Scripts   ScriptsConfig   `yaml:"scripts"`
	Alerts    AlertsConfig    `yaml:"alerts"`
	Update    UpdateConfig    `yaml:"update"`

	// path remembers where this config was loaded from so it can be rewritten
	// when the Core pushes updates.
	path string `yaml:"-"`
}

// UpdateConfig controls agent self-update. Updates are verified against a
// SHA-256 and, when a public key is configured, an Ed25519 signature — so a
// compromised download server alone cannot push malicious binaries.
type UpdateConfig struct {
	Enabled       bool   `yaml:"enabled"`
	PublicKey     string `yaml:"public_key"`     // base64 Ed25519 public key
	AllowUnsigned bool   `yaml:"allow_unsigned"` // permit updates without a signature (NOT recommended)
}

// ScriptsConfig gates remote script execution (a powerful capability, disabled
// by default; the Core enables it per-agent by pushing config).
type ScriptsConfig struct {
	Enabled             bool     `yaml:"enabled"`
	AllowedInterpreters []string `yaml:"allowed_interpreters"`
	MaxTimeoutSeconds   int      `yaml:"max_timeout_seconds"`
	WorkDir             string   `yaml:"work_dir"` // where scripts are staged; temp dir if empty
}

// AlertsConfig holds local threshold rules evaluated after each collection.
type AlertsConfig struct {
	Enabled        bool    `yaml:"enabled"`
	CPUPercent     float64 `yaml:"cpu_percent"`     // warn above this overall CPU %
	MemPercent     float64 `yaml:"mem_percent"`     // warn above this RAM used %
	DiskPercent    float64 `yaml:"disk_percent"`    // warn above this per-partition used %
	UPSBattery     float64 `yaml:"ups_battery"`     // warn when battery charge % drops below this
	ServiceStopped bool    `yaml:"service_stopped"` // alert on services in "failed" state
}

type AgentConfig struct {
	// ID is the stable identity of this install. If empty, the agent
	// generates a UUID on first run and persists it back to the config file.
	ID   string   `yaml:"id"`
	Name string   `yaml:"name"`
	Tags []string `yaml:"tags"`
}

type ServerConfig struct {
	URL       string    `yaml:"url"`       // base URL of MRTI Core, e.g. https://mrti.local
	Transport string    `yaml:"transport"` // https | websocket | mqtt
	APIKey    string    `yaml:"api_key"`
	Token     string    `yaml:"token"`
	JWT       string    `yaml:"jwt"`
	TLS       TLSConfig `yaml:"tls"`
	// Compress enables gzip compression of outbound payloads.
	Compress bool `yaml:"compression"`
}

type TLSConfig struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CACert             string `yaml:"ca_cert"`
	ClientCert         string `yaml:"client_cert"`
	ClientKey          string `yaml:"client_key"`
}

type IntervalsConfig struct {
	Collect   time.Duration `yaml:"collect"`   // how often modules run
	Heartbeat time.Duration `yaml:"heartbeat"` // how often to ping liveness
	Flush     time.Duration `yaml:"flush"`     // how often to drain the cache queue
	Commands  time.Duration `yaml:"commands"`  // how often to poll for Core commands
}

type LoggingConfig struct {
	Level     string `yaml:"level"` // debug | info | warn | error
	Dir       string `yaml:"dir"`
	MaxSizeMB int    `yaml:"max_size_mb"`
	MaxFiles  int    `yaml:"max_files"`
	Console   bool   `yaml:"console"` // also log to stdout (useful in foreground/debug)
}

type CacheConfig struct {
	Path     string `yaml:"path"`
	MaxQueue int    `yaml:"max_queue"` // max buffered envelopes before dropping oldest
}

type ModulesConfig struct {
	// Enabled lists the module names the agent should run. The Core can push
	// changes to this list to toggle modules without redeploying.
	Enabled []string `yaml:"enabled"`
	// Config holds arbitrary per-module settings keyed by module name.
	Config map[string]map[string]any `yaml:"config"`
}

type PluginsConfig struct {
	Dir     string   `yaml:"dir"`     // directory scanned for gRPC plugin binaries
	Enabled []string `yaml:"enabled"` // plugin names to load
}

// Default returns a Config populated with production-sane defaults.
func Default() *Config {
	return &Config{
		Agent: AgentConfig{},
		Server: ServerConfig{
			URL:       "https://mrti.local",
			Transport: "https",
			Compress:  true,
		},
		Intervals: IntervalsConfig{
			Collect:   30 * time.Second,
			Heartbeat: 60 * time.Second,
			Flush:     15 * time.Second,
			Commands:  30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:     "info",
			Dir:       "logs",
			MaxSizeMB: 10,
			MaxFiles:  5,
		},
		Cache: CacheConfig{
			Path:     "cache/mrti.db",
			MaxQueue: 10000,
		},
		Modules: ModulesConfig{
			Enabled: []string{"system", "cpu", "ram", "disk", "network"},
			Config:  map[string]map[string]any{},
		},
		Plugins: PluginsConfig{
			Dir:     "plugins",
			Enabled: []string{},
		},
		Scripts: ScriptsConfig{
			Enabled:             false, // safe default; enable centrally
			AllowedInterpreters: []string{"bash", "sh", "powershell", "cmd", "python", "python3"},
			MaxTimeoutSeconds:   300,
		},
		Alerts: AlertsConfig{
			Enabled:        true,
			CPUPercent:     90,
			MemPercent:     90,
			DiskPercent:    90,
			UPSBattery:     50,
			ServiceStopped: false,
		},
		Update: UpdateConfig{
			Enabled:       false, // enable centrally once a signing key is set
			AllowUnsigned: false,
		},
	}
}

// Load reads config from path, layering it over defaults. A missing file is
// not an error: defaults are used and the file is created on first Save.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := Default()
			cfg.path = path
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg, err := FromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.path = path
	return cfg, nil
}

// FromBytes parses a YAML (or JSON — JSON is valid YAML) config document over
// the built-in defaults, applying env overrides and validation. This is what
// lets the Core push a full config to an agent at runtime.
func FromBytes(data []byte) (*Config, error) {
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyEnvOverrides(cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SetPath sets the file this config should be saved to.
func (c *Config) SetPath(path string) { c.path = path }

// applyEnvOverrides lets a few sensitive/deploy-time values come from the
// environment, which is handy for containerized or scripted installs.
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("MRTI_SERVER_URL"); v != "" {
		c.Server.URL = v
	}
	if v := os.Getenv("MRTI_API_KEY"); v != "" {
		c.Server.APIKey = v
	}
	if v := os.Getenv("MRTI_TOKEN"); v != "" {
		c.Server.Token = v
	}
	if v := os.Getenv("MRTI_AGENT_ID"); v != "" {
		c.Agent.ID = v
	}
}

func (c *Config) validate() error {
	if c.Server.URL == "" {
		return fmt.Errorf("server.url is required")
	}
	switch c.Server.Transport {
	case "https", "websocket", "mqtt":
	default:
		return fmt.Errorf("server.transport %q invalid (https|websocket|mqtt)", c.Server.Transport)
	}
	if c.Intervals.Collect <= 0 {
		c.Intervals.Collect = 30 * time.Second
	}
	return nil
}

// Path returns the file this config was loaded from.
func (c *Config) Path() string { return c.path }

// Save writes the config back to its origin path (used to persist a generated
// agent ID or Core-pushed config changes).
func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("config has no path to save to")
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}

// ModuleSettings returns the raw settings map for a named module (may be nil).
func (c *Config) ModuleSettings(name string) map[string]any {
	if c.Modules.Config == nil {
		return nil
	}
	return c.Modules.Config[name]
}
