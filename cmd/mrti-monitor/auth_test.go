package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type authTransport func(*http.Request) (*http.Response, error)

func (f authTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockAuthClient(t *testing.T, transport authTransport) {
	t.Helper()
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultClient = previous })
	t.Setenv("MRTI_AUTH_MODULE_URL", "http://core.test/api/auth/module-access/agent-core")
}

func TestPortalAccessPreservesAuthorityStatus(t *testing.T) {
	for _, status := range []int{204, 401, 403, 404, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			mockAuthClient(t, func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Fatal("authorization header was not forwarded")
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
			req.Header.Set("Authorization", "Bearer fixture")
			response := httptest.NewRecorder()
			called := false
			(&server{}).requirePortalAccess(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })(response, req)
			expected := status
			if status == 204 {
				expected = 200
			} else if status != 401 && status != 403 {
				expected = 503
			}
			if response.Code != expected || called != (status == 204) {
				t.Fatalf("status=%d, called=%v; expected=%d", response.Code, called, expected)
			}
		})
	}
}

func TestPortalAccessBoundsCoreWait(t *testing.T) {
	mockAuthClient(t, func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("Core request must have a deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 5*time.Second {
			t.Fatalf("invalid authorization deadline: %s", remaining)
		}
		return nil, context.DeadlineExceeded
	})
	response := httptest.NewRecorder()
	(&server{}).requirePortalAccess(func(http.ResponseWriter, *http.Request) { t.Fatal("must not authorize on timeout") })(response, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	if response.Code != 503 {
		t.Fatalf("timeout status=%d", response.Code)
	}
}

func TestPortalAccessRespectsEarlierClientDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	expected, _ := ctx.Deadline()
	mockAuthClient(t, func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || !deadline.Equal(expected) {
			t.Fatal("the client's earlier deadline must be preserved")
		}
		return nil, context.Canceled
	})
	response := httptest.NewRecorder()
	(&server{}).requirePortalAccess(func(http.ResponseWriter, *http.Request) { t.Fatal("must not authorize canceled request") })(response, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil).WithContext(ctx))
	if response.Code != 503 {
		t.Fatalf("cancellation status=%d", response.Code)
	}
}
