// Package inventory reports the host's hardware inventory: GPUs, PCI devices,
// USB devices and attached monitors. On Linux it reads sysfs (and parses EDID
// for monitors); on Windows it queries WMI. This powers asset tracking in the
// MRTI panel.
package inventory

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("inventory", func() modules.Module { return &Module{} }) }

// PCIDevice is one device on the PCI bus.
type PCIDevice struct {
	Slot      string `json:"slot"`
	VendorID  string `json:"vendor_id"`
	DeviceID  string `json:"device_id"`
	Class     string `json:"class"`
	Vendor    string `json:"vendor,omitempty"`
	Device    string `json:"device,omitempty"`
	ClassName string `json:"class_name,omitempty"`
	Driver    string `json:"driver,omitempty"`
}

// GPU is a graphics/display adapter.
type GPU struct {
	Vendor string `json:"vendor,omitempty"`
	Model  string `json:"model,omitempty"`
	Driver string `json:"driver,omitempty"`
	Slot   string `json:"slot,omitempty"`
}

// USBDevice is one device on the USB bus.
type USBDevice struct {
	VendorID     string `json:"vendor_id"`
	ProductID    string `json:"product_id"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Product      string `json:"product,omitempty"`
	Serial       string `json:"serial,omitempty"`
}

// Monitor is an attached display.
type Monitor struct {
	Connector    string `json:"connector"`
	Connected    bool   `json:"connected"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Serial       string `json:"serial,omitempty"`
}

// Stats is the inventory payload.
type Stats struct {
	GPUs     []GPU       `json:"gpus,omitempty"`
	PCI      []PCIDevice `json:"pci,omitempty"`
	USB      []USBDevice `json:"usb,omitempty"`
	Monitors []Monitor   `json:"monitors,omitempty"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "inventory" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	stats, err := collectInventory(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(stats)
}
