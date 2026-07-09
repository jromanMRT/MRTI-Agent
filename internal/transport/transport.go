// Package transport abstracts how the agent talks to the MRTI Core. The
// orchestrator depends only on the Transport interface, so HTTPS, WebSocket
// and MQTT back-ends are interchangeable and selectable from config. HTTPS is
// fully implemented; WebSocket and MQTT are scaffolded for later.
package transport

import (
	"context"
	"fmt"

	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// Transport is the contract every back-end implements.
type Transport interface {
	// Send delivers one serialized message of the given kind
	// ("envelope"|"heartbeat"|"command_result"). It returns an error the caller
	// can use to decide whether to keep the item queued for retry.
	Send(ctx context.Context, kind string, payload []byte) error

	// Poll fetches pending commands from the Core for this agent. Back-ends
	// without a pull model (pure push) may return nil.
	Poll(ctx context.Context, agentID string) ([]model.Command, error)

	// Close releases connections/resources.
	Close() error
}

// New selects and constructs a Transport from config.
func New(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	switch cfg.Server.Transport {
	case "https":
		return newHTTPS(cfg, a)
	case "websocket":
		return newWebSocket(cfg, a)
	case "mqtt":
		return newMQTT(cfg, a)
	default:
		return nil, fmt.Errorf("unsupported transport %q", cfg.Server.Transport)
	}
}
