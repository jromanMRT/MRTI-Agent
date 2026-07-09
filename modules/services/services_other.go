//go:build !linux && !windows

package services

import "context"

// listServices is unimplemented on platforms without a service manager
// integration; returns an empty inventory.
func listServices(_ context.Context) ([]Service, error) { return nil, nil }
