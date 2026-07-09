// Package auth centralises how the agent proves its identity to the MRTI Core.
// It supports three credential styles that can be combined: a long-lived API
// key, a bearer token, and a JWT. Keeping this in one place means the
// transport layer never hand-rolls auth headers.
package auth

import (
	"net/http"

	"github.com/jromanMRT/mrti-agent/internal/config"
)

// Authenticator applies credentials to outbound requests.
type Authenticator struct {
	apiKey string
	token  string
	jwt    string
}

// New builds an Authenticator from server config.
func New(s config.ServerConfig) *Authenticator {
	return &Authenticator{
		apiKey: s.APIKey,
		token:  s.Token,
		jwt:    s.JWT,
	}
}

// Apply sets auth headers on an HTTP request. Precedence for the bearer slot:
// JWT, then token. The API key is always sent as its own header so the Core
// can identify the install even when a short-lived JWT is also present.
func (a *Authenticator) Apply(r *http.Request) {
	if a.apiKey != "" {
		r.Header.Set("X-MRTI-API-Key", a.apiKey)
	}
	switch {
	case a.jwt != "":
		r.Header.Set("Authorization", "Bearer "+a.jwt)
	case a.token != "":
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
}

// Headers returns the auth headers as a map, useful for non-HTTP transports
// (WebSocket handshake, MQTT connect properties).
func (a *Authenticator) Headers() map[string]string {
	h := map[string]string{}
	if a.apiKey != "" {
		h["X-MRTI-API-Key"] = a.apiKey
	}
	if a.jwt != "" {
		h["Authorization"] = "Bearer " + a.jwt
	} else if a.token != "" {
		h["Authorization"] = "Bearer " + a.token
	}
	return h
}

// SetJWT updates the JWT at runtime (e.g. after a token refresh handshake).
func (a *Authenticator) SetJWT(jwt string) { a.jwt = jwt }
