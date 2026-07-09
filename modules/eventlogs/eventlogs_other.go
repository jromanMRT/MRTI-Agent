//go:build !linux && !windows

package eventlogs

import (
	"context"
	"time"
)

// collectEvents is unimplemented on platforms without a supported log system.
func collectEvents(_ context.Context, _ time.Duration, _ int, _ []string) (string, []Event, error) {
	return "unsupported", nil, nil
}
