// Command mrti-agent is the MRTI monitoring agent. It runs as a background
// service on Windows and Linux, collecting host telemetry through pluggable
// modules and shipping it to the MRTI Monitor. The same binary installs,
// uninstalls and controls its own service via flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"

	"github.com/jromanMRT/mrti-agent/internal/agent"
	"github.com/jromanMRT/mrti-agent/internal/config"
	"github.com/jromanMRT/mrti-agent/internal/logging"
	"github.com/jromanMRT/mrti-agent/modules"

	// Register built-in modules so -list-modules can report them.
	_ "github.com/jromanMRT/mrti-agent/modules/cpu"
	_ "github.com/jromanMRT/mrti-agent/modules/disk"
	_ "github.com/jromanMRT/mrti-agent/modules/docker"
	_ "github.com/jromanMRT/mrti-agent/modules/eventlogs"
	_ "github.com/jromanMRT/mrti-agent/modules/inventory"
	_ "github.com/jromanMRT/mrti-agent/modules/network"
	_ "github.com/jromanMRT/mrti-agent/modules/processes"
	_ "github.com/jromanMRT/mrti-agent/modules/ram"
	_ "github.com/jromanMRT/mrti-agent/modules/services"
	_ "github.com/jromanMRT/mrti-agent/modules/snmp"
	_ "github.com/jromanMRT/mrti-agent/modules/software"
	_ "github.com/jromanMRT/mrti-agent/modules/system"
	_ "github.com/jromanMRT/mrti-agent/modules/temperature"
	_ "github.com/jromanMRT/mrti-agent/modules/ups"
	_ "github.com/jromanMRT/mrti-agent/modules/virtualization"
)

func main() {
	var (
		configPath  = flag.String("config", "", "path to config.yaml (default: config.yaml beside the binary)")
		svcAction   = flag.String("service", "", "service control action: install|uninstall|start|stop|restart")
		foreground  = flag.Bool("foreground", false, "run in foreground with console logging (do not use the service manager)")
		showVersion = flag.Bool("version", false, "print version and exit")
		listMods    = flag.Bool("list-modules", false, "list available built-in modules and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("mrti-agent %s\n", agent.Version)
		return
	}
	if *listMods {
		fmt.Println("Available built-in modules:")
		for _, m := range modules.Available() {
			fmt.Printf("  - %s\n", m)
		}
		return
	}

	cfgPath, err := prepareRuntime(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime setup error: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	console := *foreground || cfg.Logging.Console
	log, closer, err := logging.Setup(cfg.Logging.Dir, cfg.Logging.Level, cfg.Logging.MaxSizeMB, cfg.Logging.MaxFiles, console)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging error: %v\n", err)
		os.Exit(1)
	}
	defer closer.Close()

	prg := &program{cfg: cfg, log: log}

	svcConfig := &service.Config{
		Name:        "mrti-agent",
		DisplayName: "MRTI Agent",
		Description: "MRTI infrastructure monitoring agent",
		Arguments:   []string{"-config", cfgPath},
		Option: service.KeyValue{
			// Windows starts the agent at boot and restarts it after a failure.
			// These keys are ignored by service managers that do not support them.
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   86400,
		},
	}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "service init error: %v\n", err)
		os.Exit(1)
	}

	// Service control actions (install/uninstall/start/stop/restart).
	if *svcAction != "" {
		if err := service.Control(svc, *svcAction); err != nil {
			fmt.Fprintf(os.Stderr, "service %s failed: %v\n", *svcAction, err)
			os.Exit(1)
		}
		fmt.Printf("service %s: ok\n", *svcAction)
		return
	}

	// Foreground mode runs the program directly (Ctrl-C to stop); otherwise the
	// service manager drives Start/Stop.
	if *foreground {
		if err := prg.run(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "run error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := svc.Run(); err != nil {
		log.Error("service run error", "err", err)
		os.Exit(1)
	}
}

// program adapts the agent to the kardianos service lifecycle.
type program struct {
	cfg    *config.Config
	log    *slog.Logger
	cancel context.CancelFunc
	done   chan struct{}
}

// Start is called by the service manager; it must not block.
func (p *program) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	ag, err := agent.New(p.cfg, p.log)
	if err != nil {
		cancel()
		return fmt.Errorf("initialize agent: %w", err)
	}
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := ag.Run(ctx); err != nil {
			p.log.Error("agent exited with error", "err", err)
		}
	}()
	return nil
}

// Stop is called by the service manager on shutdown.
func (p *program) Stop(_ service.Service) error {
	p.log.Info("service stop signal received")
	if p.cancel != nil {
		p.cancel()
	}
	select {
	case <-p.done:
	case <-time.After(25 * time.Second):
		p.log.Warn("agent did not stop within grace period")
	}
	return nil
}

// run builds and runs the agent until ctx is cancelled.
func (p *program) run(ctx context.Context) error {
	ag, err := agent.New(p.cfg, p.log)
	if err != nil {
		return err
	}
	return ag.Run(ctx)
}

// resolveConfigPath picks the config file: the -config flag if given, else
// config.yaml next to the executable, else config.yaml in the working dir.
func resolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if exe, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(exe), "config.yaml")
		if _, err := os.Stat(beside); err == nil {
			return beside
		}
	}
	return "config.yaml"
}

// prepareRuntime makes the config path absolute and uses its directory as the
// process working directory. Windows services otherwise start in System32,
// which would put relative logs/cache/plugin paths in the wrong location.
func prepareRuntime(flagPath string) (string, error) {
	cfgPath, err := filepath.Abs(resolveConfigPath(flagPath))
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.Chdir(filepath.Dir(cfgPath)); err != nil {
		return "", fmt.Errorf("use config directory: %w", err)
	}
	return cfgPath, nil
}
