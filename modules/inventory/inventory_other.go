//go:build !linux && !windows

package inventory

import "context"

// collectInventory is unimplemented on platforms without a sysfs/WMI backend.
func collectInventory(_ context.Context) (Stats, error) { return Stats{}, nil }
