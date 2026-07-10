package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// mqttTransport publishes telemetry to and receives commands from an MQTT
// broker — useful for constrained networks or fan-in fleet topologies where a
// broker aggregates thousands of agents. Telemetry is published to
// mrti/<agentID>/<kind>; commands arrive on mrti/<agentID>/commands. The Paho
// client handles reconnection transparently.
type mqttTransport struct {
	client    mqtt.Client
	agentID   string
	qos       byte
	connect   sync.Once
	connErr   error
	baseTopic string

	cmdMu sync.Mutex
	cmds  []model.Command
}

func newMQTT(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	tlsCfg, err := buildTLS(cfg.Server.TLS)
	if err != nil {
		return nil, err
	}

	agentID := cfg.Agent.ID
	t := &mqttTransport{
		agentID:   agentID,
		qos:       1, // at-least-once
		baseTopic: "mrti/" + agentID,
	}

	opts := mqtt.NewClientOptions().
		AddBroker(toMQTTBroker(cfg.Server.URL)).
		SetClientID("mrti-agent-" + agentID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectTimeout(15 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetCleanSession(false).
		SetOnConnectHandler(t.onConnect)

	// Credentials: API key as username, token/JWT as password.
	headers := cfg.Server
	if headers.APIKey != "" {
		opts.SetUsername(headers.APIKey)
	}
	if headers.JWT != "" {
		opts.SetPassword(headers.JWT)
	} else if headers.Token != "" {
		opts.SetPassword(headers.Token)
	}
	if strings.HasPrefix(opts.Servers[0].Scheme, "ssl") || strings.HasPrefix(opts.Servers[0].Scheme, "tls") {
		opts.SetTLSConfig(tlsCfg)
	}

	t.client = mqtt.NewClient(opts)
	_ = a // auth creds already pulled from config above
	return t, nil
}

// toMQTTBroker normalises a URL to a Paho broker address. http(s) is mapped to
// tcp/ssl; explicit tcp/ssl/mqtt/mqtts/ws schemes pass through. A bare host
// gets a default tcp:// scheme and :1883 port.
func toMQTTBroker(u string) string {
	u = strings.TrimSpace(u)
	switch {
	case strings.HasPrefix(u, "https://"):
		return "ssl://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		return "tcp://" + strings.TrimPrefix(u, "http://")
	case strings.HasPrefix(u, "mqtts://"):
		return "ssl://" + strings.TrimPrefix(u, "mqtts://")
	case strings.HasPrefix(u, "mqtt://"):
		return "tcp://" + strings.TrimPrefix(u, "mqtt://")
	case strings.Contains(u, "://"):
		return u // tcp://, ssl://, ws:// already
	default:
		return "tcp://" + u
	}
}

// onConnect (re)subscribes to the command topic whenever the client connects,
// so subscriptions survive reconnects.
func (t *mqttTransport) onConnect(c mqtt.Client) {
	c.Subscribe(t.baseTopic+"/commands", t.qos, t.handleCommandMessage)
}

func (t *mqttTransport) handleCommandMessage(_ mqtt.Client, msg mqtt.Message) {
	// A command message may be a single Command or an array.
	var one model.Command
	if err := json.Unmarshal(msg.Payload(), &one); err == nil && one.Type != "" {
		t.bufferCommands(one)
		return
	}
	var many []model.Command
	if err := json.Unmarshal(msg.Payload(), &many); err == nil {
		t.bufferCommands(many...)
	}
}

func (t *mqttTransport) bufferCommands(cmds ...model.Command) {
	t.cmdMu.Lock()
	t.cmds = append(t.cmds, cmds...)
	t.cmdMu.Unlock()
}

// ensureConnected connects the client once (subsequent reconnects are handled
// by Paho).
func (t *mqttTransport) ensureConnected() error {
	t.connect.Do(func() {
		tok := t.client.Connect()
		if !tok.WaitTimeout(20 * time.Second) {
			t.connErr = fmt.Errorf("mqtt connect timeout")
			return
		}
		t.connErr = tok.Error()
	})
	if t.connErr != nil {
		return t.connErr
	}
	if !t.client.IsConnected() {
		return fmt.Errorf("mqtt not connected")
	}
	return nil
}

func (t *mqttTransport) Send(ctx context.Context, kind string, payload []byte) error {
	if err := t.ensureConnected(); err != nil {
		return err
	}
	topic := t.baseTopic + "/" + kind
	tok := t.client.Publish(topic, t.qos, false, payload)
	if !tok.WaitTimeout(15 * time.Second) {
		return fmt.Errorf("mqtt publish timeout on %s", topic)
	}
	return tok.Error()
}

func (t *mqttTransport) Poll(_ context.Context, _ string) ([]model.Command, error) {
	if err := t.ensureConnected(); err != nil {
		return nil, err
	}
	t.cmdMu.Lock()
	defer t.cmdMu.Unlock()
	if len(t.cmds) == 0 {
		return nil, nil
	}
	out := t.cmds
	t.cmds = nil
	return out, nil
}

func (t *mqttTransport) Close() error {
	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(500)
	}
	return nil
}

// compile-time assertion that mqttTransport satisfies Transport.
var _ Transport = (*mqttTransport)(nil)
