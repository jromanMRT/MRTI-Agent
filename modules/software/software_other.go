//go:build !linux && !windows

package software

import "context"

// listSoftware is unimplemented on platforms without a package manager
// integration.
func listSoftware(_ context.Context) (string, []Program, error) {
	return "unknown", nil, nil
}
