package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// TestWebSocketRoundTrip verifies the WS transport delivers an outbound frame
// and surfaces a command the server pushes back.
func TestWebSocketRoundTrip(t *testing.T) {
	received := make(chan []byte, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the auth header rode along on the handshake.
		if r.Header.Get("X-MRTI-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "done")

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Read the client's first frame (the envelope) and echo it to the test.
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		received <- data

		// Push a command back to the agent.
		cmd, _ := json.Marshal(model.Command{ID: "c1", Type: "ping"})
		f, _ := json.Marshal(frame{Kind: "command", Data: cmd})
		_ = c.Write(ctx, websocket.MessageText, f)

		// Keep the connection open briefly so the client can read.
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.Server.URL = srv.URL
	cfg.Server.Transport = "websocket"
	cfg.Server.APIKey = "test-key"

	tr, err := newWebSocket(cfg, auth.New(cfg.Server))
	if err != nil {
		t.Fatalf("newWebSocket: %v", err)
	}
	defer tr.Close()

	ctx := context.Background()
	payload := []byte(`{"schema":"mrti.v1","sequence":1}`)
	if err := tr.Send(ctx, "envelope", payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Server should have received our framed envelope.
	select {
	case got := <-received:
		var f frame
		if err := json.Unmarshal(got, &f); err != nil {
			t.Fatalf("server got non-frame: %v", err)
		}
		if f.Kind != "envelope" || string(f.Data) != string(payload) {
			t.Fatalf("unexpected frame: kind=%s data=%s", f.Kind, f.Data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received the envelope")
	}

	// The pushed command should surface via Poll (read loop is async).
	var cmds []model.Command
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cmds, _ = tr.Poll(ctx, "agent")
		if len(cmds) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(cmds) != 1 || cmds[0].ID != "c1" || cmds[0].Type != "ping" {
		t.Fatalf("expected pushed command c1/ping, got %+v", cmds)
	}
}

// TestToWSURL checks scheme conversion and path.
func TestToWSURL(t *testing.T) {
	cases := map[string]string{
		"https://mrti.local":     "wss://mrti.local/api/v1/ws",
		"http://127.0.0.1:8099/": "ws://127.0.0.1:8099/api/v1/ws",
		"wss://already.ws":       "wss://already.ws/api/v1/ws",
	}
	for in, want := range cases {
		if got := toWSURL(in); got != want {
			t.Errorf("toWSURL(%q) = %q, want %q", in, got, want)
		}
	}
}
