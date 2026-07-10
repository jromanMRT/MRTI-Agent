//go:build linux

package inventory

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// collectInventory gathers PCI/GPU/USB/monitor inventory from sysfs.
func collectInventory(_ context.Context) (Stats, error) {
	var s Stats
	s.PCI, s.GPUs = readPCI()
	s.USB = readUSB()
	s.Monitors = readMonitors()
	return s, nil
}

// --- PCI + GPU ---

func readPCI() ([]PCIDevice, []GPU) {
	base := "/sys/bus/pci/devices"
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, nil
	}
	vendors, devices := loadPCIIDs()

	var pci []PCIDevice
	var gpus []GPU
	for _, e := range entries {
		slot := e.Name()
		dir := filepath.Join(base, slot)
		vid := trimHex(readSysfs(filepath.Join(dir, "vendor")))
		did := trimHex(readSysfs(filepath.Join(dir, "device")))
		class := trimHex(readSysfs(filepath.Join(dir, "class")))
		driver := ""
		if link, err := os.Readlink(filepath.Join(dir, "driver")); err == nil {
			driver = filepath.Base(link)
		}

		d := PCIDevice{
			Slot: slot, VendorID: vid, DeviceID: did, Class: class, Driver: driver,
			Vendor:    vendors[vid],
			Device:    devices[vid+did],
			ClassName: className(class),
		}
		pci = append(pci, d)

		// Display controllers (class 0x03xxxx) are GPUs.
		if strings.HasPrefix(class, "03") {
			gpus = append(gpus, GPU{
				Vendor: d.Vendor, Model: d.Device, Driver: driver, Slot: slot,
			})
		}
	}
	return pci, gpus
}

// className maps the top-level PCI class byte to a human name.
func className(class string) string {
	if len(class) < 2 {
		return ""
	}
	switch class[:2] {
	case "00":
		return "Unclassified"
	case "01":
		return "Mass storage controller"
	case "02":
		return "Network controller"
	case "03":
		return "Display controller"
	case "04":
		return "Multimedia controller"
	case "05":
		return "Memory controller"
	case "06":
		return "Bridge"
	case "07":
		return "Communication controller"
	case "08":
		return "Base system peripheral"
	case "09":
		return "Input device controller"
	case "0b":
		return "Processor"
	case "0c":
		return "Serial bus controller"
	case "0d":
		return "Wireless controller"
	default:
		return ""
	}
}

var (
	pciIDsOnce    sync.Once
	pciVendorsMap map[string]string
	pciDevicesMap map[string]string
)

// loadPCIIDs parses the system pci.ids database (once) to resolve vendor and
// device names. Absence is fine — names are just left blank.
func loadPCIIDs() (map[string]string, map[string]string) {
	pciIDsOnce.Do(func() {
		pciVendorsMap = map[string]string{}
		pciDevicesMap = map[string]string{}
		var f *os.File
		var err error
		for _, p := range []string{"/usr/share/misc/pci.ids", "/usr/share/hwdata/pci.ids"} {
			if f, err = os.Open(p); err == nil {
				break
			}
		}
		if f == nil {
			return
		}
		defer f.Close()

		var curVendor string
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "C ") { // class section begins; stop
				break
			}
			switch {
			case !strings.HasPrefix(line, "\t"): // vendor line: "8086  Intel..."
				id, name, ok := splitIDName(line)
				if ok {
					curVendor = id
					pciVendorsMap[id] = name
				}
			case strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "\t\t"): // device
				id, name, ok := splitIDName(strings.TrimPrefix(line, "\t"))
				if ok && curVendor != "" {
					pciDevicesMap[curVendor+id] = name
				}
			}
		}
	})
	return pciVendorsMap, pciDevicesMap
}

// splitIDName parses "abcd  Some Name" into ("abcd", "Some Name").
func splitIDName(line string) (string, string, bool) {
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return "", "", false
	}
	id := line[:i]
	name := strings.TrimSpace(line[i:])
	if len(id) != 4 {
		return "", "", false
	}
	return id, name, true
}

// --- USB ---

func readUSB() []USBDevice {
	base := "/sys/bus/usb/devices"
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []USBDevice
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		vid := readSysfs(filepath.Join(dir, "idVendor"))
		if vid == "" { // interfaces (e.g. "1-0:1.0") have no idVendor
			continue
		}
		out = append(out, USBDevice{
			VendorID:     vid,
			ProductID:    readSysfs(filepath.Join(dir, "idProduct")),
			Manufacturer: readSysfs(filepath.Join(dir, "manufacturer")),
			Product:      readSysfs(filepath.Join(dir, "product")),
			Serial:       readSysfs(filepath.Join(dir, "serial")),
		})
	}
	return out
}

// --- Monitors (DRM connectors + EDID) ---

func readMonitors() []Monitor {
	base := "/sys/class/drm"
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []Monitor
	for _, e := range entries {
		name := e.Name()
		if !strings.Contains(name, "-") { // skip "card1", keep "card1-HDMI-A-1"
			continue
		}
		dir := filepath.Join(base, name)
		status := readSysfs(filepath.Join(dir, "status"))
		connector := connectorName(name)
		mon := Monitor{Connector: connector, Connected: status == "connected"}
		if mon.Connected {
			if edid, err := os.ReadFile(filepath.Join(dir, "edid")); err == nil && len(edid) >= 128 {
				mon.Manufacturer, mon.Model, mon.Serial = parseEDID(edid)
			}
		}
		out = append(out, mon)
	}
	return out
}

// connectorName strips the "cardN-" prefix, leaving e.g. "HDMI-A-1".
func connectorName(name string) string {
	if i := strings.IndexByte(name, '-'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// parseEDID extracts the manufacturer PnP ID and the monitor name/serial from
// the EDID descriptor blocks.
func parseEDID(b []byte) (mfr, model, serial string) {
	// Manufacturer: bytes 8-9, three 5-bit letters (A=1).
	m := uint16(b[8])<<8 | uint16(b[9])
	mfr = string([]byte{
		byte((m>>10)&0x1f) + 'A' - 1,
		byte((m>>5)&0x1f) + 'A' - 1,
		byte(m&0x1f) + 'A' - 1,
	})

	// Four 18-byte descriptors at offsets 54, 72, 90, 108.
	for _, off := range []int{54, 72, 90, 108} {
		if off+18 > len(b) {
			break
		}
		d := b[off : off+18]
		if d[0] != 0 || d[1] != 0 || d[2] != 0 { // not a text descriptor
			continue
		}
		text := strings.TrimRight(strings.SplitN(string(d[5:18]), "\n", 2)[0], " \x00")
		switch d[3] {
		case 0xFC: // monitor name
			model = strings.TrimSpace(text)
		case 0xFF: // serial number
			serial = strings.TrimSpace(text)
		}
	}
	return mfr, model, serial
}

// --- helpers ---

func readSysfs(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func trimHex(s string) string { return strings.TrimPrefix(s, "0x") }
