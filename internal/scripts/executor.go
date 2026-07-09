// Package scripts executes remote scripts pushed by the MRTI Core (from
// MRTOps): it stages the script content to a temp file, runs it under the
// requested interpreter with a hard timeout, and captures stdout/stderr/exit
// code. Execution is gated by config (disabled by default) and restricted to
// an allow-list of interpreters, since this is a high-privilege capability.
package scripts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jromanMRT/mrti-agent/internal/config"
)

// Request is the payload of a "run_script" command.
type Request struct {
	Interpreter    string   `json:"interpreter"` // bash|sh|powershell|cmd|python|python3
	Script         string   `json:"script"`      // inline script content
	Args           []string `json:"args,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
	WorkingDir     string   `json:"working_dir,omitempty"`
}

// Result is returned to the Core after execution.
type Result struct {
	Interpreter string `json:"interpreter"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	DurationMS  int64  `json:"duration_ms"`
	TimedOut    bool   `json:"timed_out"`
	Error       string `json:"error,omitempty"`
}

// Executor runs scripts within the bounds of the configured policy.
type Executor struct {
	cfg config.ScriptsConfig
}

// NewExecutor builds an Executor from config.
func NewExecutor(cfg config.ScriptsConfig) *Executor { return &Executor{cfg: cfg} }

// Enabled reports whether remote script execution is permitted.
func (e *Executor) Enabled() bool { return e.cfg.Enabled }

// Run executes req, enforcing the interpreter allow-list and timeout ceiling.
func (e *Executor) Run(ctx context.Context, req Request) Result {
	res := Result{Interpreter: req.Interpreter, ExitCode: -1}

	if !e.cfg.Enabled {
		res.Error = "remote script execution is disabled on this agent"
		return res
	}
	if !e.allowed(req.Interpreter) {
		res.Error = fmt.Sprintf("interpreter %q not allowed", req.Interpreter)
		return res
	}

	timeout := e.effectiveTimeout(req.TimeoutSeconds)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	file, cleanup, err := e.stage(req)
	if err != nil {
		res.Error = "stage script: " + err.Error()
		return res
	}
	defer cleanup()

	name, args, err := buildCommand(req.Interpreter, file, req.Args)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	cmd := exec.CommandContext(runCtx, name, args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	res.DurationMS = time.Since(start).Milliseconds()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	if runCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.Error = "script timed out"
		return res
	}
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.Error = runErr.Error()
		}
		return res
	}
	res.ExitCode = 0
	return res
}

func (e *Executor) allowed(interp string) bool {
	for _, a := range e.cfg.AllowedInterpreters {
		if a == interp {
			return true
		}
	}
	return false
}

func (e *Executor) effectiveTimeout(reqSeconds int) time.Duration {
	max := e.cfg.MaxTimeoutSeconds
	if max <= 0 {
		max = 300
	}
	s := reqSeconds
	if s <= 0 || s > max {
		s = max
	}
	return time.Duration(s) * time.Second
}

// stage writes the script to a temp file with an interpreter-appropriate
// extension and returns a cleanup func.
func (e *Executor) stage(req Request) (string, func(), error) {
	dir := e.cfg.WorkDir
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "mrti-script-*"+ext(req.Interpreter))
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	cleanup := func() { os.Remove(name) }
	if _, err := f.WriteString(req.Script); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	// Make executable on Unix (harmless on Windows).
	_ = os.Chmod(name, 0o700)
	return name, cleanup, nil
}

func ext(interp string) string {
	switch interp {
	case "powershell":
		return ".ps1"
	case "cmd":
		return ".bat"
	case "python", "python3":
		return ".py"
	default:
		return ".sh"
	}
}

// buildCommand resolves the interpreter binary and its invocation for a staged
// script file.
func buildCommand(interp, file string, args []string) (string, []string, error) {
	var name string
	var pre []string
	switch interp {
	case "bash":
		name = "bash"
		pre = []string{file}
	case "sh":
		name = "sh"
		pre = []string{file}
	case "python", "python3":
		name = interp
		pre = []string{file}
	case "powershell":
		name = powershellBinary()
		pre = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", file}
	case "cmd":
		if runtime.GOOS != "windows" {
			return "", nil, fmt.Errorf("cmd interpreter is Windows-only")
		}
		name = "cmd"
		pre = []string{"/C", file}
	default:
		return "", nil, fmt.Errorf("unsupported interpreter %q", interp)
	}

	if path, err := exec.LookPath(name); err == nil {
		name = path
	} else {
		return "", nil, fmt.Errorf("interpreter %q not found on host: %w", interp, err)
	}
	return name, append(pre, args...), nil
}

// powershellBinary prefers PowerShell 7+ (pwsh) then Windows PowerShell.
func powershellBinary() string {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh"
	}
	return "powershell"
}

// staging path helper kept exported-adjacent for tests.
var _ = filepath.Base
