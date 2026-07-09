// Package modules defines the contract every collection module implements and
// a global registry of built-in (native, compiled-in) modules. External gRPC
// plugins live outside this package but are adapted to the same Module
// interface by internal/pluginhost, so the agent orchestrator treats native
// modules and plugins uniformly.
package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

// Module is a single unit of data collection. Implementations must be safe to
// call Collect repeatedly and should be cheap: the agent's whole value
// proposition is being light. A Module should return quickly and respect
// context cancellation.
type Module interface {
	// Name is the stable identifier used in config's enabledModules and as the
	// key in the outbound payload.
	Name() string

	// Configure is called once before collection starts with the module's
	// settings block from config and a scoped logger. It may be a no-op.
	Configure(settings map[string]any, log *slog.Logger) error

	// Collect gathers data for one cycle and returns it as JSON. Returning an
	// error is recorded per-module without aborting the whole cycle.
	Collect(ctx context.Context) (json.RawMessage, error)

	// Close releases any resources held by the module.
	Close() error
}

// Factory constructs a fresh Module instance.
type Factory func() Module

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register makes a native module available by name. Called from module
// package init() functions. Panics on duplicate names to catch build-time
// mistakes early.
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("modules: duplicate registration for %q", name))
	}
	registry[name] = f
}

// New instantiates a registered native module, or returns an error if unknown.
func New(name string) (Module, error) {
	regMu.RLock()
	f, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown module %q", name)
	}
	return f(), nil
}

// Available returns the sorted list of registered native module names.
func Available() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// BaseModule is an embeddable helper that provides a no-op Configure/Close and
// stores a logger, so simple modules only need to implement Name and Collect.
type BaseModule struct {
	Log *slog.Logger
}

func (b *BaseModule) Configure(_ map[string]any, log *slog.Logger) error {
	b.Log = log
	return nil
}

func (b *BaseModule) Close() error { return nil }
