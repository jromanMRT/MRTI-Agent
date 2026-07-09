// Package logging configures the agent's structured logger. It writes JSON
// logs to a rotating file under the configured log directory and optionally
// mirrors them to stdout when running in the foreground. Using the standard
// library's log/slog keeps the dependency surface small and the output
// machine-parseable for the MRTI Core to ingest later.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// rotatingWriter is a minimal size-based log rotator so the agent stays light
// (no third-party rotation dependency). When the active file exceeds maxSize
// it is renamed with a numeric suffix and a fresh file is opened.
type rotatingWriter struct {
	mu       sync.Mutex
	dir      string
	base     string
	maxSize  int64
	maxFiles int
	size     int64
	f        *os.File
}

func newRotatingWriter(dir, base string, maxSizeMB, maxFiles int) (*rotatingWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &rotatingWriter{
		dir:      dir,
		base:     base,
		maxSize:  int64(maxSizeMB) * 1024 * 1024,
		maxFiles: maxFiles,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	path := filepath.Join(w.dir, w.base)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		w.rotate()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() {
	w.f.Close()
	// Shift older files: base.(n-1) -> base.n, dropping the oldest.
	for i := w.maxFiles - 1; i >= 1; i-- {
		older := filepath.Join(w.dir, w.base+"."+itoa(i))
		newer := filepath.Join(w.dir, w.base+"."+itoa(i+1))
		os.Rename(older, newer)
	}
	if w.maxFiles > 0 {
		os.Rename(filepath.Join(w.dir, w.base), filepath.Join(w.dir, w.base+".1"))
	}
	w.open()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// Setup builds a *slog.Logger writing to logs/mrti-agent.log with optional
// console mirroring. Returns the logger and a closer for graceful shutdown.
func Setup(dir, level string, maxSizeMB, maxFiles int, console bool) (*slog.Logger, io.Closer, error) {
	rw, err := newRotatingWriter(dir, "mrti-agent.log", maxSizeMB, maxFiles)
	if err != nil {
		return nil, nil, err
	}

	var out io.Writer = rw
	if console {
		out = io.MultiWriter(rw, os.Stdout)
	}

	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	handler := slog.NewJSONHandler(out, opts)
	logger := slog.New(handler)
	return logger, rw, nil
}

// Close implements io.Closer for the rotating writer.
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		return w.f.Close()
	}
	return nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
