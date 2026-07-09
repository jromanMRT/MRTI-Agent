//go:build windows

package system

import "github.com/yusufpapurcu/wmi"

// readDMI resolves hardware identity through WMI. The wmi package is already
// pulled in transitively by gopsutil on Windows, so this adds no new
// dependency weight. Errors are swallowed to keep the module non-fatal.
func readDMI() dmiInfo {
	var d dmiInfo

	var bios []struct {
		SerialNumber      string
		Manufacturer      string
		SMBIOSBIOSVersion string
	}
	if err := wmi.Query("SELECT SerialNumber, Manufacturer, SMBIOSBIOSVersion FROM Win32_BIOS", &bios); err == nil && len(bios) > 0 {
		d.Serial = bios[0].SerialNumber
		d.BIOS = bios[0].Manufacturer + " " + bios[0].SMBIOSBIOSVersion
	}

	var cs []struct {
		Manufacturer string
		Model        string
		Domain       string
	}
	if err := wmi.Query("SELECT Manufacturer, Model, Domain FROM Win32_ComputerSystem", &cs); err == nil && len(cs) > 0 {
		d.Manufacturer = cs[0].Manufacturer
		d.Model = cs[0].Model
		d.Domain = cs[0].Domain
	}
	return d
}
