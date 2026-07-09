package transport

import (
	"context"
	"fmt"

	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// mqttTransport is a placeholder for an MQTT back-end (e.g. for constrained or
// fan-in fleet topologies). A real implementation would publish envelopes to
// topics like mrti/<agentID>/telemetry and subscribe to mrti/<agentID>/commands.
// Stubbed for now so config can already declare "mqtt" as prepared.
type mqttTransport struct {
	broker string
	auth   *auth.Authenticator
}

func newMQTT(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	return &mqttTransport{broker: cfg.Server.URL, auth: a}, nil
}

func (m *mqttTransport) Send(ctx context.Context, kind string, payload []byte) error {
	return fmt.Errorf("mqtt transport not yet implemented")
}

func (m *mqttTransport) Poll(ctx context.Context, agentID string) ([]model.Command, error) {
	return nil, nil
}

func (m *mqttTransport) Close() error { return nil }
