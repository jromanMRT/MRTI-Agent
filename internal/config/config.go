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

	// path remembers where this config was loaded from so it can be rewritten
	// when the Core pushes updates.
	path string `yaml:"-"`
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
	}
}

// Load reads config from path, layering it over defaults. A missing file is
// not an error: defaults are used and the file is created on first Save.
func Load(path string) (*Config, error) {
	cfg := Default()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.path = path
	applyEnvOverrides(cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

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
