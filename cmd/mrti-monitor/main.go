// Command mrti-monitor is a reference MRTI Monitor server. It receives telemetry from
// MRTI agents, stores the latest state per agent in SQLite, and exposes it as a
// JSON REST API, a Prometheus /metrics endpoint and a live HTML dashboard. It
// also queues commands for agents to pick up. This is a self-hostable starting
// point for the MRTI platform — point an agent's server.url at it and go.
package main

import (
	"compress/gzip"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/model"
)

// basePathKey carries the external prefix ("" for direct :8477 access used by
// telemetry agents and legacy bookmarks, "/agent-core" when reached through
// nginx at the same origin as the rest of the platform) so the dashboard can
// emit correctly prefixed self-references. It never touches the agent-facing
// ingest/commands routes, which must keep working unprefixed on :8477.
type basePathKeyType struct{}

var basePathKey basePathKeyType

// rootHandler wraps coreHandler so the exact same routes answer both at the
// server's own root (unchanged — telemetry agents point their server.url
// directly at :8477 and must never be affected by this) and, stripped, under
// "/agent-core/" for nginx to reverse-proxy at the platform's main origin.
func rootHandler(store *Store, apiKey, downloadsDir string) http.Handler {
	inner := coreHandler(store, apiKey, downloadsDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		basePath := ""
		path := r.URL.Path
		if path == "/agent-core" || strings.HasPrefix(path, "/agent-core/") {
			basePath = "/agent-core"
			path = strings.TrimPrefix(path, "/agent-core")
			if path == "" {
				path = "/"
			}
		}
		r2 := r.WithContext(context.WithValue(r.Context(), basePathKey, basePath))
		r2.URL.Path = path
		inner.ServeHTTP(w, r2)
	})
}

func coreHandler(store *Store, apiKey, downloadsDir string) http.Handler {
	srv := &server{store: store, apiKey: apiKey, downloadsDir: downloadsDir}
	mux := http.NewServeMux()

	// Agent-facing endpoints.
	mux.HandleFunc("POST /api/v1/ingest", srv.ingest)
	mux.HandleFunc("GET /api/v1/agents/{id}/commands", srv.getCommands)

	// Operator/API endpoints.
	mux.HandleFunc("GET /api/v1/agents", srv.requirePortalAccess(srv.listAgents))
	mux.HandleFunc("GET /api/v1/agents/{id}", srv.requirePortalAccess(srv.getAgent))
	mux.HandleFunc("PATCH /api/v1/agents/{id}", srv.requireOperatorKey(srv.patchAgent))
	mux.HandleFunc("DELETE /api/v1/agents/{id}", srv.requireOperatorKey(srv.deleteAgent))
	mux.HandleFunc("GET /api/v1/agents/{id}/modules/{module}", srv.requirePortalAccess(srv.getModule))
	mux.HandleFunc("POST /api/v1/agents/{id}/commands", srv.requirePortalAccess(srv.postCommand))
	mux.HandleFunc("GET /api/v1/alerts", srv.requirePortalAccess(srv.getAlerts))
	mux.HandleFunc("GET /api/v1/export", srv.requirePortalAccess(srv.export))
	mux.HandleFunc("GET /metrics", srv.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /downloads/", srv.downloadsPage)
	mux.HandleFunc("GET /downloads/files/{name}", srv.downloadFile)
	mux.HandleFunc("GET /portal-assets/{name}", portalAsset)

	// Dashboard.
	mux.HandleFunc("GET /", srv.dashboard)

	return logRequests(mux)
}

type server struct {
	store        *Store
	apiKey       string
	downloadsDir string
}

func (s *server) requireOperatorKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-MRTI-API-Key")
		if s.apiKey != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.apiKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *server) requirePortalAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authURL := os.Getenv("MRTI_AUTH_MODULE_URL")
		if authURL == "" {
			authURL = "http://127.0.0.1:3002/api/auth/module-access/agent-core"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
		if err != nil {
			http.Error(w, "authorization unavailable", http.StatusServiceUnavailable)
			return
		}
		request.Header.Set("Authorization", r.Header.Get("Authorization"))
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			http.Error(w, "authorization unavailable", http.StatusServiceUnavailable)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			status := response.StatusCode
			if status != http.StatusUnauthorized && status != http.StatusForbidden {
				status = http.StatusServiceUnavailable
			}
			http.Error(w, "unauthorized", status)
			return
		}
		next(w, r)
	}
}

// ingest accepts envelopes, heartbeats and command results from agents.
func (s *server) ingest(w http.ResponseWriter, r *http.Request) {
	if s.apiKey != "" && r.Header.Get("X-MRTI-API-Key") != s.apiKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := readBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Header.Get("X-MRTI-Kind") {
	case "heartbeat":
		var hb model.Heartbeat
		if json.Unmarshal(body, &hb) == nil {
			s.store.IngestHeartbeat(hb)
		}
	case "command_result":
		var res model.CommandResult
		if json.Unmarshal(body, &res) == nil {
			s.store.SaveCommandResult(res)
		}
	default: // "envelope"
		var env model.Envelope
		if json.Unmarshal(body, &env) == nil {
			s.store.IngestEnvelope(env)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// getCommands delivers queued commands to an agent (and marks them delivered).
func (s *server) getCommands(w http.ResponseWriter, r *http.Request) {
	cmds := s.store.TakeCommands(r.PathValue("id"))
	if len(cmds) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, cmds)
}

func (s *server) listAgents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListAgents())
}

func (s *server) getAgent(w http.ResponseWriter, r *http.Request) {
	raw, ok := s.store.GetEnvelope(r.PathValue("id"))
	if !ok {
		http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

func (s *server) patchAgent(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request struct {
		Archived *bool `json:"archived"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.Archived == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must include archived"})
		return
	}
	found, err := s.store.SetAgentArchived(r.PathValue("id"), *request.Archived)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "archived": *request.Archived})
}

func (s *server) deleteAgent(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.DeleteAgent(r.PathValue("id"))
	if errors.Is(err, ErrAgentOnline) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "disconnect the agent before deleting it"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) getModule(w http.ResponseWriter, r *http.Request) {
	raw, ok := s.store.GetModule(r.PathValue("id"), r.PathValue("module"))
	if !ok {
		http.Error(w, `{"error":"no data for that agent/module"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// postCommand enqueues a command for an agent. Body: {"type":"...","payload":{...}}.
func (s *server) postCommand(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var cmd model.Command
	if err := json.Unmarshal(body, &cmd); err != nil || cmd.Type == "" {
		http.Error(w, `{"error":"body must be a command with a type"}`, http.StatusBadRequest)
		return
	}
	if cmd.ID == "" {
		cmd.ID = "cmd-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	cmd.IssuedAt = time.Now()
	s.store.EnqueueCommand(r.PathValue("id"), cmd)
	writeJSON(w, http.StatusAccepted, map[string]string{"queued": cmd.ID})
}

func (s *server) getAlerts(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, s.store.RecentAlerts(limit))
}

func (s *server) export(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=mrti-export.json")
	w.Write(mustJSON(s.store.ExportAll()))
}

// --- helpers ---

func readBody(r *http.Request) ([]byte, error) {
	var reader io.Reader = http.MaxBytesReader(nil, r.Body, 16<<20)
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(reader)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(mustJSON(v))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.URL.Path != "/metrics" && r.URL.Path != "/" {
			log.Printf("%s %s", r.Method, r.URL.Path)
		}
	})
}

func portOnly(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return addr
	}
	if i := lastColon(addr); i >= 0 {
		return addr[i:]
	}
	return addr
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
