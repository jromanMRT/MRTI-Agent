//go:build linux

package virtualization

import (
	"os"
	"strings"
)

// extraDetect adds Linux-specific virtualization signals that gopsutil's
// generic detection doesn't surface (Proxmox host, WSL, systemd-nspawn).
func extraDetect() []string {
	var hints []string

	// Proxmox VE stores cluster/config under /etc/pve.
	if fi, err := os.Stat("/etc/pve"); err == nil && fi.IsDir() {
		hints = append(hints, "proxmox")
	}
	// WSL exposes "microsoft" in the kernel version string.
	if b, err := os.ReadFile("/proc/version"); err == nil {
		if strings.Contains(strings.ToLower(string(b)), "microsoft") {
			hints = append(hints, "wsl")
		}
	}
	// systemd-nspawn / container marker.
	if _, err := os.Stat("/run/systemd/container"); err == nil {
		hints = append(hints, "systemd-container")
	}
	// Docker-in-host marker.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		hints = append(hints, "dockerenv")
	}
	return hints
}
