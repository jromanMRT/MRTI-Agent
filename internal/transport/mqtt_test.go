package transport

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	mauth "github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// freePort returns a currently-free TCP address on localhost.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// startBroker spins up an embedded MQTT broker for the test.
func startBroker(t *testing.T, addr string) *mochi.Server {
	t.Helper()
	server := mochi.New(&mochi.Options{InlineClient: true})
	if err := server.AddHook(new(mauth.AllowHook), nil); err != nil {
		t.Fatal(err)
	}
	if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "t1", Address: addr})); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	time.Sleep(200 * time.Millisecond) // let the listener bind
	return server
}

func TestMQTTRoundTrip(t *testing.T) {
	addr := freePort(t)
	server := startBroker(t, addr)
	defer server.Close()

	// Broker-side subscription to capture the agent's published telemetry.
	got := make(chan []byte, 1)
	err := server.Subscribe("mrti/test-agent/envelope", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		select {
		case got <- pk.Payload:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Server.Transport = "mqtt"
	cfg.Server.URL = "tcp://" + addr
	cfg.Agent.ID = "test-agent"

	tr, err := newMQTT(cfg, auth.New(cfg.Server))
	if err != nil {
		t.Fatalf("newMQTT: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()
	payload := []byte(`{"schema":"mrti.v1","sequence":7}`)
	if err := tr.Send(ctx, "envelope", payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case p := <-got:
		if string(p) != string(payload) {
			t.Fatalf("broker got %q, want %q", p, payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("broker never received the published envelope")
	}

	// Give the agent's command subscription time to establish, then push a
	// command from the broker and verify Poll surfaces it.
	time.Sleep(300 * time.Millisecond)
	cmd, _ := json.Marshal(model.Command{ID: "m1", Type: "ping"})
	if err := server.Publish("mrti/test-agent/commands", cmd, false, 1); err != nil {
		t.Fatal(err)
	}

	var cmds []model.Command
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cmds, _ = tr.Poll(ctx, "test-agent")
		if len(cmds) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(cmds) != 1 || cmds[0].ID != "m1" || cmds[0].Type != "ping" {
		t.Fatalf("expected pushed command m1/ping, got %+v", cmds)
	}
}

func TestToMQTTBroker(t *testing.T) {
	cases := map[string]string{
		"https://broker:8883": "ssl://broker:8883",
		"http://broker:1883":  "tcp://broker:1883",
		"mqtts://broker":      "ssl://broker",
		"mqtt://broker":       "tcp://broker",
		"tcp://broker:1883":   "tcp://broker:1883",
		"broker:1883":         "tcp://broker:1883",
	}
	for in, want := range cases {
		if got := toMQTTBroker(in); got != want {
			t.Errorf("toMQTTBroker(%q) = %q, want %q", in, got, want)
		}
	}
}
