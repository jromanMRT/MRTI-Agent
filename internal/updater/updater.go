// Package updater implements verified agent self-update. Given an update
// descriptor (download URL, SHA-256, optional Ed25519 signature), it downloads
// the new binary, verifies its integrity and authenticity, and atomically
// swaps it in with a .bak rollback. The actual process restart is left to the
// caller/service manager (the agent exits so systemd / Windows SC relaunches
// the new binary).
package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/config"
)

// maxBinarySize caps the download to guard against a hostile/huge response.
const maxBinarySize = 200 * 1024 * 1024 // 200 MB

// Request is the update descriptor, typically the payload of an "update"
// command from the Core.
type Request struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`              // hex-encoded
	Signature string `json:"signature,omitempty"` // base64 Ed25519 signature over the binary bytes
}

// Result reports the outcome of an update attempt.
type Result struct {
	Version    string `json:"version"`
	Applied    bool   `json:"applied"`
	RolledBack bool   `json:"rolled_back,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Updater applies verified updates according to policy.
type Updater struct {
	cfg    config.UpdateConfig
	client *http.Client
}

// New builds an Updater. If client is nil a default is used.
func New(cfg config.UpdateConfig, client *http.Client) *Updater {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Updater{cfg: cfg, client: client}
}

// Enabled reports whether self-update is permitted.
func (u *Updater) Enabled() bool { return u.cfg.Enabled }

// Apply downloads, verifies and installs the update at targetPath (the running
// binary). It never leaves targetPath in a broken state: on any failure after
// the swap it restores the backup.
func (u *Updater) Apply(ctx context.Context, req Request, targetPath string) Result {
	res := Result{Version: req.Version, TargetPath: targetPath}

	if !u.cfg.Enabled {
		res.Error = "self-update is disabled on this agent"
		return res
	}
	if req.URL == "" || req.SHA256 == "" {
		res.Error = "update requires url and sha256"
		return res
	}

	data, err := u.download(ctx, req.URL)
	if err != nil {
		res.Error = "download: " + err.Error()
		return res
	}

	if err := verifyChecksum(data, req.SHA256); err != nil {
		res.Error = err.Error()
		return res
	}
	if err := u.verifySignature(data, req.Signature); err != nil {
		res.Error = err.Error()
		return res
	}

	if err := swapBinary(targetPath, data); err != nil {
		res.Error = err.Error()
		if err2 := restoreBackup(targetPath); err2 == nil {
			res.RolledBack = true
		}
		return res
	}

	res.Applied = true
	return res
}

// download fetches the URL into memory (bounded), so the whole payload is
// available for hashing and signature verification before it touches disk.
func (u *Updater) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBinarySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBinarySize {
		return nil, fmt.Errorf("update exceeds max size of %d bytes", maxBinarySize)
	}
	return data, nil
}

func verifyChecksum(data []byte, wantHex string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !equalFold(got, wantHex) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, wantHex)
	}
	return nil
}

// verifySignature enforces the signing policy: if a public key is configured
// and a signature is provided, it must verify; if either is missing, the update
// is rejected unless AllowUnsigned is set.
func (u *Updater) verifySignature(data []byte, sigB64 string) error {
	if u.cfg.PublicKey == "" || sigB64 == "" {
		if u.cfg.AllowUnsigned {
			return nil
		}
		return fmt.Errorf("update rejected: signature required (set update.public_key and sign the binary, or allow_unsigned)")
	}
	pub, err := base64.StdEncoding.DecodeString(u.cfg.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid update.public_key")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

// swapBinary writes the new bytes beside the target, then renames into place
// after backing up the current binary to <target>.bak (same directory, so the
// rename is atomic on one filesystem).
func swapBinary(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".mrti-update-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp: %w", err)
	}

	backup := target + ".bak"
	os.Remove(backup) // clear any stale backup
	if err := os.Rename(target, backup); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}

// restoreBackup rolls back to <target>.bak if the install failed.
func restoreBackup(target string) error {
	backup := target + ".bak"
	if _, err := os.Stat(backup); err != nil {
		return err
	}
	return os.Rename(backup, target)
}

// equalFold compares two hex strings case-insensitively without allocating.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 32
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
