//go:build linux

package system

import (
	"os"
	"strings"
)

// readDMI reads hardware identity from the sysfs DMI tree. Some fields
// (product_serial, product_uuid) require root; unreadable fields are simply
// left empty so the agent still works unprivileged.
func readDMI() dmiInfo {
	read := func(name string) string {
		b, err := os.ReadFile("/sys/class/dmi/id/" + name)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	d := dmiInfo{
		Serial:       read("product_serial"),
		Model:        read("product_name"),
		Manufacturer: read("sys_vendor"),
		BIOS:         strings.TrimSpace(read("bios_vendor") + " " + read("bios_version")),
	}
	// NIS/DNS domain, best effort.
	if b, err := os.ReadFile("/proc/sys/kernel/domainname"); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" && v != "(none)" {
			d.Domain = v
		}
	}
	return d
}
