package transport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// frame is the envelope used on the wire in both directions. Outbound kinds are
// "envelope"|"heartbeat"|"command_result"; inbound the Core sends "command"
// (data is a model.Command) or "commands" (data is a []model.Command).
type frame struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// wsTransport keeps a single persistent WebSocket connection to the Core,
// multiplexing outbound telemetry and inbound pushed commands over it. It
// reconnects lazily with a backoff so a dropped link doesn't spin the CPU, and
// it satisfies the same Transport interface as the HTTPS back-end: Send writes
// a frame, Poll drains commands the read loop has buffered.
type wsTransport struct {
	url    string
	auth   *auth.Authenticator
	tls    *tls.Config
	dialTO time.Duration

	mu        sync.Mutex
	conn      *websocket.Conn
	connected bool
	nextRetry time.Time

	cmdMu sync.Mutex
	cmds  []model.Command
}

func newWebSocket(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	tlsCfg, err := buildTLS(cfg.Server.TLS)
	if err != nil {
		return nil, err
	}
	return &wsTransport{
		url:    toWSURL(cfg.Server.URL),
		auth:   a,
		tls:    tlsCfg,
		dialTO: 15 * time.Second,
	}, nil
}

// toWSURL converts an http(s) base URL to a ws(s) endpoint at /api/v1/ws.
func toWSURL(base string) string {
	b := strings.TrimRight(base, "/")
	switch {
	case strings.HasPrefix(b, "https://"):
		b = "wss://" + strings.TrimPrefix(b, "https://")
	case strings.HasPrefix(b, "http://"):
		b = "ws://" + strings.TrimPrefix(b, "http://")
	}
	return b + "/api/v1/ws"
}

// Send delivers one framed message, connecting first if necessary. A write
// error tears down the connection so the next call reconnects; the caller
// keeps the item queued for retry.
func (w *wsTransport) Send(ctx context.Context, kind string, payload []byte) error {
	conn, err := w.ensureConn(ctx)
	if err != nil {
		return err
	}
	data, err := json.Marshal(frame{Kind: kind, Data: json.RawMessage(payload)})
	if err != nil {
		return err
	}
	// Serialize writes: a websocket connection allows only one concurrent writer.
	w.mu.Lock()
	writeErr := conn.Write(ctx, websocket.MessageText, data)
	w.mu.Unlock()
	if writeErr != nil {
		w.disconnect(writeErr)
		return writeErr
	}
	return nil
}

// Poll ensures the link is up (so commands can be pushed) and returns any
// commands the read loop has buffered since the last call.
func (w *wsTransport) Poll(ctx context.Context, _ string) ([]model.Command, error) {
	if _, err := w.ensureConn(ctx); err != nil {
		return nil, err
	}
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()
	if len(w.cmds) == 0 {
		return nil, nil
	}
	out := w.cmds
	w.cmds = nil
	return out, nil
}

// ensureConn returns a live connection, dialing (with backoff) if needed.
func (w *wsTransport) ensureConn(ctx context.Context) (*websocket.Conn, error) {
	w.mu.Lock()
	if w.connected && w.conn != nil {
		c := w.conn
		w.mu.Unlock()
		return c, nil
	}
	if time.Now().Before(w.nextRetry) {
		w.mu.Unlock()
		return nil, fmt.Errorf("websocket backoff until %s", w.nextRetry.Format(time.RFC3339))
	}
	w.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(ctx, w.dialTO)
	defer cancel()

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{},
		HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: w.tls}},
	}
	for k, v := range w.auth.Headers() {
		opts.HTTPHeader.Set(k, v)
	}

	conn, _, err := websocket.Dial(dialCtx, w.url, opts)
	if err != nil {
		w.mu.Lock()
		w.nextRetry = time.Now().Add(10 * time.Second)
		w.mu.Unlock()
		return nil, fmt.Errorf("websocket dial %s: %w", w.url, err)
	}
	// Large messages (full inventories) can exceed the default read limit.
	conn.SetReadLimit(8 << 20)

	w.mu.Lock()
	w.conn = conn
	w.connected = true
	w.nextRetry = time.Time{}
	w.mu.Unlock()

	go w.readLoop(conn)
	return conn, nil
}

// readLoop consumes inbound frames and buffers any pushed commands until the
// connection drops.
func (w *wsTransport) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			w.disconnect(err)
			return
		}
		var f frame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Kind {
		case "command":
			var cmd model.Command
			if json.Unmarshal(f.Data, &cmd) == nil {
				w.bufferCommands(cmd)
			}
		case "commands":
			var cmds []model.Command
			if json.Unmarshal(f.Data, &cmds) == nil {
				w.bufferCommands(cmds...)
			}
		}
	}
}

func (w *wsTransport) bufferCommands(cmds ...model.Command) {
	w.cmdMu.Lock()
	w.cmds = append(w.cmds, cmds...)
	w.cmdMu.Unlock()
}

// disconnect marks the connection down (and closes it) so the next Send/Poll
// reconnects. Safe to call multiple times.
func (w *wsTransport) disconnect(cause error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.conn.Close(websocket.StatusInternalError, "closing")
		w.conn = nil
	}
	if w.connected {
		w.connected = false
		// brief backoff before the next reconnect attempt
		w.nextRetry = time.Now().Add(3 * time.Second)
	}
}

func (w *wsTransport) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.conn.Close(websocket.StatusNormalClosure, "shutdown")
		w.conn = nil
	}
	w.connected = false
	return nil
}
