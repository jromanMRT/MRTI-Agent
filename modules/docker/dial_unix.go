//go:build !windows

package docker

import (
	"context"
	"net"
	"strings"
)

// defaultDockerHost is the standard Docker socket path on Unix.
func defaultDockerHost() string { return "/var/run/docker.sock" }

// dialDocker connects to the Docker daemon Unix socket. It accepts either a
// bare path or a unix://<path> form.
func dialDocker(ctx context.Context, host string) (net.Conn, error) {
	path := strings.TrimPrefix(host, "unix://")
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}
