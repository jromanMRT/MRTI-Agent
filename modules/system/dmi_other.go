//go:build !linux && !windows

package system

// readDMI is a no-op on platforms without a DMI/WMI implementation (e.g.
// macOS/BSD). Hardware fields are left empty.
func readDMI() dmiInfo { return dmiInfo{} }
