// Package system collects host identity and platform information: hostname,
// OS, kernel, architecture, uptime, UUID, virtualization and — where the
// platform allows — hardware serial/model/manufacturer/BIOS via DMI.
package system

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/host"
)

func init() { modules.Register("system", func() modules.Module { return &Module{} }) }

// Info is the payload this module emits each cycle.
type Info struct {
	Hostname       string    `json:"hostname"`
	Domain         string    `json:"domain,omitempty"`
	OS             string    `json:"os"`       // linux, windows, ...
	Platform       string    `json:"platform"` // ubuntu, debian, "Microsoft Windows 11 Pro"
	Version        string    `json:"version"`  // platform version
	Kernel         string    `json:"kernel"`   // kernel version
	Arch           string    `json:"arch"`     // x86_64, arm64
	UUID           string    `json:"uuid"`     // stable host id
	Serial         string    `json:"serial,omitempty"`
	Model          string    `json:"model,omitempty"`
	Manufacturer   string    `json:"manufacturer,omitempty"`
	BIOS           string    `json:"bios,omitempty"`
	Virtualization string    `json:"virtualization,omitempty"` // kvm, vmware, hyperv, docker...
	VirtRole       string    `json:"virtualization_role,omitempty"`
	BootTime       time.Time `json:"boot_time"`
	Uptime         uint64    `json:"uptime_seconds"`
	Timezone       string    `json:"timezone"`
	LocalTime      time.Time `json:"local_time"`
	Processes      uint64    `json:"processes"`
}

// Module implements modules.Module for host info.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "system" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	h, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	tz, _ := time.Now().Zone()
	dmi := readDMI() // platform-specific, best-effort

	info := Info{
		Hostname:       h.Hostname,
		OS:             h.OS,
		Platform:       h.Platform,
		Version:        h.PlatformVersion,
		Kernel:         h.KernelVersion,
		Arch:           h.KernelArch,
		UUID:           h.HostID,
		Virtualization: h.VirtualizationSystem,
		VirtRole:       h.VirtualizationRole,
		BootTime:       time.Unix(int64(h.BootTime), 0),
		Uptime:         h.Uptime,
		Timezone:       tz,
		LocalTime:      time.Now(),
		Processes:      h.Procs,
		Serial:         dmi.Serial,
		Model:          dmi.Model,
		Manufacturer:   dmi.Manufacturer,
		BIOS:           dmi.BIOS,
		Domain:         dmi.Domain,
	}
	return json.Marshal(info)
}

// dmiInfo carries the hardware fields resolved by platform-specific code.
type dmiInfo struct {
	Serial       string
	Model        string
	Manufacturer string
	BIOS         string
	Domain       string
}
