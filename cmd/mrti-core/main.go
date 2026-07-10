// Command mrti-core is a reference MRTI Core server. It receives telemetry from
// MRTI agents, stores the latest state per agent in SQLite, and exposes it as a
// JSON REST API, a Prometheus /metrics endpoint and a live HTML dashboard. It
// also queues commands for agents to pick up. This is a self-hostable starting
// point for the MRTI platform — point an agent's server.url at it and go.
package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/model"
)

func main() {
	addr := flag.String("addr", ":8477", "listen address")
	dbPath := flag.String("db", "core.db", "path to the SQLite database")
	apiKey := flag.String("api-key", "demo-api-key", "API key agents must present on ingest (X-MRTI-API-Key)")
	flag.Parse()

	store, err := openStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	srv := &server{store: store, apiKey: *apiKey}
	mux := http.NewServeMux()

	// Agent-facing endpoints.
	mux.HandleFunc("POST /api/v1/ingest", srv.ingest)
	mux.HandleFunc("GET /api/v1/agents/{id}/commands", srv.getCommands)

	// Operator/API endpoints.
	mux.HandleFunc("GET /api/v1/agents", srv.listAgents)
	mux.HandleFunc("GET /api/v1/agents/{id}", srv.getAgent)
	mux.HandleFunc("GET /api/v1/agents/{id}/modules/{module}", srv.getModule)
	mux.HandleFunc("POST /api/v1/agents/{id}/commands", srv.postCommand)
	mux.HandleFunc("GET /api/v1/alerts", srv.getAlerts)
	mux.HandleFunc("GET /api/v1/export", srv.export)
	mux.HandleFunc("GET /metrics", srv.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Dashboard.
	mux.HandleFunc("GET /", srv.dashboard)

	handler := logRequests(mux)
	log.Printf("MRTI Core listening on %s  (db=%s)", *addr, *dbPath)
	log.Printf("  dashboard : http://localhost%s/", portOnly(*addr))
	log.Printf("  API       : http://localhost%s/api/v1/agents", portOnly(*addr))
	log.Printf("  metrics   : http://localhost%s/metrics", portOnly(*addr))
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	store  *Store
	apiKey string
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
