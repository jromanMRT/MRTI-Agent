package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/service"
)

type coreOptions struct {
	addr         string
	dbPath       string
	apiKey       string
	downloadsDir string
}

type coreProgram struct {
	opts coreOptions

	mu       sync.Mutex
	http     *http.Server
	listener net.Listener
	store    *Store
	done     chan struct{}
}

func main() {
	addr := flag.String("addr", ":8477", "listen address")
	dbPath := flag.String("db", "core.db", "path to the SQLite database")
	apiKey := flag.String("api-key", "demo-api-key", "API key agents must present on ingest (X-MRTI-API-Key)")
	downloadsDir := flag.String("downloads-dir", "dist/downloads", "directory containing public agent installer packages")
	svcAction := flag.String("service", "", "service control action: install|uninstall|start|stop|restart")
	foreground := flag.Bool("foreground", false, "run attached to the current console")
	flag.Parse()

	dbAbs, err := filepath.Abs(*dbPath)
	if err != nil {
		fatalf("resolve database path: %v", err)
	}
	downloadsAbs, err := filepath.Abs(*downloadsDir)
	if err != nil {
		fatalf("resolve downloads directory: %v", err)
	}
	opts := coreOptions{addr: *addr, dbPath: dbAbs, apiKey: *apiKey, downloadsDir: downloadsAbs}
	program := &coreProgram{opts: opts}

	config := &service.Config{
		Name:        "mrti-core",
		DisplayName: "MRTI Core",
		Description: "MRTI monitoring server, API and dashboard",
		Arguments: []string{
			"-addr", opts.addr,
			"-db", opts.dbPath,
			"-api-key", opts.apiKey,
			"-downloads-dir", opts.downloadsDir,
		},
		Option: service.KeyValue{
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   86400,
		},
	}
	svc, err := service.New(program, config)
	if err != nil {
		fatalf("service init error: %v", err)
	}

	if *svcAction != "" {
		if err := service.Control(svc, *svcAction); err != nil {
			fatalf("service %s failed: %v", *svcAction, err)
		}
		fmt.Printf("service %s: ok\n", *svcAction)
		return
	}

	logFile, err := openCoreLog(filepath.Join(filepath.Dir(opts.dbPath), "mrti-core.log"), *foreground)
	if err != nil {
		fatalf("open log: %v", err)
	}
	defer logFile.Close()

	if *foreground {
		if err := program.Start(svc); err != nil {
			fatalf("start: %v", err)
		}
		<-program.done
		return
	}
	if err := svc.Run(); err != nil {
		fatalf("service run error: %v", err)
	}
}

func openCoreLog(path string, includeConsole bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	var output io.Writer = f
	if includeConsole {
		output = io.MultiWriter(os.Stdout, f)
	}
	log.SetOutput(output)
	return f, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// Start opens the database and listening socket before returning so Windows
// reports a failed service start when either resource is unavailable.
func (p *coreProgram) Start(_ service.Service) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	store, err := openStore(p.opts.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	listener, err := net.Listen("tcp", p.opts.addr)
	if err != nil {
		store.Close()
		return fmt.Errorf("listen on %s: %w", p.opts.addr, err)
	}

	p.store = store
	p.listener = listener
	p.http = &http.Server{
		Addr:              p.opts.addr,
		Handler:           coreHandler(store, p.opts.apiKey, p.opts.downloadsDir),
		ReadHeaderTimeout: 10 * time.Second,
	}
	p.done = make(chan struct{})

	log.Printf("MRTI Core listening on %s (db=%s)", p.opts.addr, p.opts.dbPath)
	log.Printf("dashboard: http://localhost%s/", portOnly(p.opts.addr))
	log.Printf("downloads: http://localhost%s/downloads/ (dir=%s)", portOnly(p.opts.addr), p.opts.downloadsDir)
	go func() {
		defer close(p.done)
		if err := p.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("MRTI Core stopped unexpectedly: %v", err)
			// Ending the process lets the Windows Service Control Manager apply
			// the configured restart-on-failure policy.
			os.Exit(1)
		}
	}()
	return nil
}

func (p *coreProgram) Stop(_ service.Service) error {
	p.mu.Lock()
	httpServer := p.http
	store := p.store
	done := p.done
	p.mu.Unlock()

	if httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := httpServer.Shutdown(ctx)
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	if store != nil {
		if closeErr := store.Close(); err == nil {
			err = closeErr
		}
	}
	log.Printf("MRTI Core stopped")
	return err
}
