//go:build !linux

package virtualization

// extraDetect has no additional signals beyond gopsutil's detection on
// non-Linux platforms yet.
func extraDetect() []string { return nil }
