package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRootHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := openStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return rootHandler(store, "", t.TempDir())
}

// The telemetry agents deployed across the network point their server.url
// directly at :8477 and must keep working unprefixed, exactly as before this
// change — this is the one thing that must never regress.
func TestRootHandlerServesUnprefixedRoutesUnchanged(t *testing.T) {
	handler := newTestRootHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("unprefixed /healthz: status=%d body=%q", response.Code, response.Body.String())
	}
}

// Same routes must also answer under /agent-core/, stripped, for nginx to
// reverse-proxy at the platform's shared origin.
func TestRootHandlerStripsAgentCorePrefix(t *testing.T) {
	handler := newTestRootHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-core/healthz", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("prefixed /agent-core/healthz: status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestRootHandlerStripsAgentCoreBareRoot(t *testing.T) {
	handler := newTestRootHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-core", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "<!doctype html>") {
		t.Fatalf("bare /agent-core did not serve the dashboard: status=%d", response.Code)
	}
}

// The dashboard must self-reference assets/API calls with whichever prefix
// it was actually reached through, so links and fetches resolve correctly
// from the browser's point of view in both access modes.
func TestDashboardEmitsBasePathAwareSelfReferences(t *testing.T) {
	handler := newTestRootHandler(t)

	direct := httptest.NewRecorder()
	handler.ServeHTTP(direct, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(direct.Body.String(), `src="/portal-assets/company-logo.svg"`) {
		t.Fatal("direct access must reference /portal-assets/... unprefixed")
	}
	if strings.Contains(direct.Body.String(), "/agent-core/portal-assets") {
		t.Fatal("direct access must not leak the /agent-core prefix")
	}

	proxied := httptest.NewRecorder()
	handler.ServeHTTP(proxied, httptest.NewRequest(http.MethodGet, "/agent-core/", nil))
	if !strings.Contains(proxied.Body.String(), `src="/agent-core/portal-assets/company-logo.svg"`) {
		t.Fatal("proxied access must reference /agent-core/portal-assets/...")
	}
	if !strings.Contains(proxied.Body.String(), `const AGENT_BASE = '/agent-core';`) {
		t.Fatal("proxied access must set AGENT_BASE for its own API calls")
	}

	if !strings.Contains(direct.Body.String(), `const AGENT_BASE = '';`) {
		t.Fatal("direct access must set an empty AGENT_BASE")
	}
}

func TestRootHandlerServesPortalAssetsUnderPrefix(t *testing.T) {
	handler := newTestRootHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-core/portal-assets/favicon.svg", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("prefixed portal-assets: status=%d", response.Code)
	}
	if ct := response.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
}
