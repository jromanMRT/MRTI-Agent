package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jromanMRT/mrti-agent/internal/config"
)

// serve returns a test server that hands out the given bytes at /bin.
func serve(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	}))
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func writeTarget(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "mrti-agent")
	if err := os.WriteFile(target, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestApply_SignedSuccess(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	newBin := []byte("NEW-BINARY-v2")
	srv := serve(t, newBin)
	defer srv.Close()

	target := writeTarget(t, "OLD-BINARY-v1")
	u := New(config.UpdateConfig{
		Enabled:   true,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}, srv.Client())

	res := u.Apply(context.Background(), Request{
		Version:   "2.0.0",
		URL:       srv.URL + "/bin",
		SHA256:    sha256hex(newBin),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, newBin)),
	}, target)

	if !res.Applied || res.Error != "" {
		t.Fatalf("expected applied, got %+v", res)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "NEW-BINARY-v2" {
		t.Fatalf("target not swapped: %q", got)
	}
	// Backup should hold the previous binary.
	bak, _ := os.ReadFile(target + ".bak")
	if string(bak) != "OLD-BINARY-v1" {
		t.Fatalf("backup missing/incorrect: %q", bak)
	}
}

func TestApply_BadChecksumRejected(t *testing.T) {
	newBin := []byte("NEW-BINARY")
	srv := serve(t, newBin)
	defer srv.Close()

	target := writeTarget(t, "OLD")
	u := New(config.UpdateConfig{Enabled: true, AllowUnsigned: true}, srv.Client())

	res := u.Apply(context.Background(), Request{
		URL:    srv.URL + "/bin",
		SHA256: sha256hex([]byte("something-else")),
	}, target)

	if res.Applied || res.Error == "" {
		t.Fatalf("expected checksum rejection, got %+v", res)
	}
	if got, _ := os.ReadFile(target); string(got) != "OLD" {
		t.Fatalf("target should be untouched, got %q", got)
	}
}

func TestApply_UnsignedRejectedByPolicy(t *testing.T) {
	newBin := []byte("NEW")
	srv := serve(t, newBin)
	defer srv.Close()

	target := writeTarget(t, "OLD")
	// Signing required (no public key, AllowUnsigned false) → must reject.
	u := New(config.UpdateConfig{Enabled: true}, srv.Client())

	res := u.Apply(context.Background(), Request{
		URL:    srv.URL + "/bin",
		SHA256: sha256hex(newBin),
	}, target)

	if res.Applied || res.Error == "" {
		t.Fatalf("expected unsigned rejection, got %+v", res)
	}
}

func TestApply_BadSignatureRejected(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	newBin := []byte("NEW")
	srv := serve(t, newBin)
	defer srv.Close()

	target := writeTarget(t, "OLD")
	u := New(config.UpdateConfig{
		Enabled:   true,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}, srv.Client())

	res := u.Apply(context.Background(), Request{
		URL:       srv.URL + "/bin",
		SHA256:    sha256hex(newBin),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(wrongPriv, newBin)),
	}, target)

	if res.Applied || res.Error == "" {
		t.Fatalf("expected signature rejection, got %+v", res)
	}
}

func TestApply_DisabledRejected(t *testing.T) {
	u := New(config.UpdateConfig{Enabled: false}, http.DefaultClient)
	res := u.Apply(context.Background(), Request{URL: "http://x", SHA256: "y"}, "/tmp/x")
	if res.Applied || res.Error == "" {
		t.Fatalf("expected disabled rejection, got %+v", res)
	}
}
