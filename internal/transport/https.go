package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/auth"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/model"
)

// httpsTransport ships messages to the Core over HTTPS REST.
type httpsTransport struct {
	base     string
	client   *http.Client
	auth     *auth.Authenticator
	compress bool
}

func newHTTPS(cfg *config.Config, a *auth.Authenticator) (Transport, error) {
	tlsCfg, err := buildTLS(cfg.Server.TLS)
	if err != nil {
		return nil, err
	}
	return &httpsTransport{
		base:     strings.TrimRight(cfg.Server.URL, "/"),
		compress: cfg.Server.Compress,
		auth:     a,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				MaxIdleConns:        4,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}, nil
}

// buildTLS assembles a *tls.Config from config: optional custom CA, optional
// client certificate for mutual TLS, and an escape hatch for self-signed dev
// servers.
func buildTLS(c config.TLSConfig) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // opt-in for dev
	}
	if c.CACert != "" {
		pem, err := os.ReadFile(c.CACert)
		if err != nil {
			return nil, fmt.Errorf("read ca_cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca_cert %s: no certificates parsed", c.CACert)
		}
		t.RootCAs = pool
	}
	if c.ClientCert != "" && c.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCert, c.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}
	return t, nil
}

func (h *httpsTransport) Send(ctx context.Context, kind string, payload []byte) error {
	url := h.base + "/api/v1/ingest"
	body := payload
	var enc string
	if h.compress {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(payload); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		body = buf.Bytes()
		enc = "gzip"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MRTI-Kind", kind)
	if enc != "" {
		req.Header.Set("Content-Encoding", enc)
	}
	h.auth.Apply(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer drain(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("ingest %s: server returned %s", kind, resp.Status)
}

func (h *httpsTransport) Poll(ctx context.Context, agentID string) ([]model.Command, error) {
	url := fmt.Sprintf("%s/api/v1/agents/%s/commands", h.base, agentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	h.auth.Apply(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer drain(resp.Body)

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("poll commands: server returned %s", resp.Status)
	}
	var cmds []model.Command
	if err := json.NewDecoder(resp.Body).Decode(&cmds); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	return cmds, nil
}

func (h *httpsTransport) Close() error {
	h.client.CloseIdleConnections()
	return nil
}

// drain reads and closes a response body so the connection can be reused.
func drain(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 1<<16))
	_ = rc.Close()
}
