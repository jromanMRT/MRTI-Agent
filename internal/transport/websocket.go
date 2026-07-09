package transport

import (
	"context"
	"fmt"

	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// wsTransport is a placeholder for a persistent WebSocket back-end. A real
// implementation would keep a single upgraded connection open, multiplex
// outbound envelopes and inbound commands over it, and reconnect with backoff.
// It is stubbed so the transport selector and config already understand
// "websocket" without pulling in a WS dependency for the MVP.
type wsTransport struct {
	base string
	auth *auth.Authenticator
}

func newWebSocket(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	return &wsTransport{base: cfg.Server.URL, auth: a}, nil
}

func (w *wsTransport) Send(ctx context.Context, kind string, payload []byte) error {
	return fmt.Errorf("websocket transport not yet implemented")
}

func (w *wsTransport) Poll(ctx context.Context, agentID string) ([]model.Command, error) {
	return nil, nil
}

func (w *wsTransport) Close() error { return nil }
