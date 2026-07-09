//go:build windows

package docker

import (
	"context"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

// defaultDockerHost is the Docker Desktop / Engine named pipe on Windows.
func defaultDockerHost() string { return `\\.\pipe\docker_engine` }

// dialDocker connects to the Docker daemon named pipe. It accepts either a
// bare pipe path or an npipe://<path> form.
func dialDocker(ctx context.Context, host string) (net.Conn, error) {
	path := strings.TrimPrefix(host, "npipe://")
	return winio.DialPipeContext(ctx, path)
}
