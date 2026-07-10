//go:build windows

package inventory

import (
	"context"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

// collectInventory gathers hardware inventory via WMI on Windows.
func collectInventory(_ context.Context) (Stats, error) {
	var s Stats
	s.GPUs = readGPUs()
	s.PCI = readPnP("PCI")
	s.USB = readUSB()
	s.Monitors = readMonitors()
	return s, nil
}

func readGPUs() []GPU {
	var rows []struct {
		Name                 string
		AdapterCompatibility string
		DriverVersion        string
	}
	q := "SELECT Name, AdapterCompatibility, DriverVersion FROM Win32_VideoController"
	if err := wmi.Query(q, &rows); err != nil {
		return nil
	}
	out := make([]GPU, 0, len(rows))
	for _, r := range rows {
		out = append(out, GPU{Vendor: r.AdapterCompatibility, Model: r.Name, Driver: r.DriverVersion})
	}
	return out
}

// readPnP lists Win32_PnPEntity rows whose DeviceID starts with the given bus
// prefix ("PCI" or "USB"), extracting vendor/device IDs from the DeviceID.
func readPnP(prefix string) []PCIDevice {
	var rows []struct {
		Name         string
		Manufacturer string
		DeviceID     string
	}
	q := "SELECT Name, Manufacturer, DeviceID FROM Win32_PnPEntity WHERE DeviceID LIKE '" + prefix + "%'"
	if err := wmi.Query(q, &rows); err != nil {
		return nil
	}
	out := make([]PCIDevice, 0, len(rows))
	for _, r := range rows {
		vid, did := parseIDs(r.DeviceID)
		out = append(out, PCIDevice{
			Slot:     r.DeviceID,
			VendorID: vid, DeviceID: did,
			Vendor: r.Manufacturer, Device: r.Name,
		})
	}
	return out
}

func readUSB() []USBDevice {
	var rows []struct {
		Name         string
		Manufacturer string
		DeviceID     string
	}
	q := "SELECT Name, Manufacturer, DeviceID FROM Win32_PnPEntity WHERE DeviceID LIKE 'USB%'"
	if err := wmi.Query(q, &rows); err != nil {
		return nil
	}
	out := make([]USBDevice, 0, len(rows))
	for _, r := range rows {
		vid, pid := parseIDs(r.DeviceID)
		out = append(out, USBDevice{
			VendorID: vid, ProductID: pid,
			Manufacturer: r.Manufacturer, Product: r.Name,
		})
	}
	return out
}

// readMonitors decodes WmiMonitorID from the root\wmi namespace.
func readMonitors() []Monitor {
	var rows []struct {
		InstanceName     string
		ManufacturerName []int32
		UserFriendlyName []int32
		SerialNumberID   []int32
		Active           bool
	}
	q := "SELECT InstanceName, ManufacturerName, UserFriendlyName, SerialNumberID, Active FROM WmiMonitorID"
	if err := wmi.QueryNamespace(q, &rows, "root\\wmi"); err != nil {
		return nil
	}
	out := make([]Monitor, 0, len(rows))
	for _, r := range rows {
		out = append(out, Monitor{
			Connector:    r.InstanceName,
			Connected:    r.Active,
			Manufacturer: decodeWMIString(r.ManufacturerName),
			Model:        decodeWMIString(r.UserFriendlyName),
			Serial:       decodeWMIString(r.SerialNumberID),
		})
	}
	return out
}

// decodeWMIString converts a WMI uint16 char-code array to a string.
func decodeWMIString(codes []int32) string {
	var b strings.Builder
	for _, c := range codes {
		if c == 0 {
			break
		}
		b.WriteRune(rune(c))
	}
	return strings.TrimSpace(b.String())
}

// parseIDs extracts vendor and device/product IDs from a PnP DeviceID like
// "PCI\VEN_8086&DEV_9B53&..." or "USB\VID_046D&PID_C52B\...".
func parseIDs(deviceID string) (string, string) {
	upper := strings.ToUpper(deviceID)
	extract := func(key string) string {
		i := strings.Index(upper, key)
		if i < 0 {
			return ""
		}
		rest := upper[i+len(key):]
		end := strings.IndexAny(rest, "&\\")
		if end < 0 {
			end = len(rest)
		}
		return strings.ToLower(rest[:end])
	}
	vid := extract("VEN_")
	if vid == "" {
		vid = extract("VID_")
	}
	did := extract("DEV_")
	if did == "" {
		did = extract("PID_")
	}
	return vid, did
}
