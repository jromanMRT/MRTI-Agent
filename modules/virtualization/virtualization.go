// Package virtualization detects whether the host runs inside a hypervisor or
// container (and which one), and whether it itself acts as a hypervisor. This
// classifies fleet assets as bare-metal, VM, or container hosts.
package virtualization

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
	"github.com/shirou/gopsutil/v4/host"
)

func init() { modules.Register("virtualization", func() modules.Module { return &Module{} }) }

// Stats is the virtualization payload.
type Stats struct {
	System       string   `json:"system,omitempty"` // kvm, vmware, hyperv, virtualbox, xen, docker, lxc, ...
	Role         string   `json:"role,omitempty"`   // guest | host
	IsVirtual    bool     `json:"is_virtual"`       // running inside a VM/container
	IsHypervisor bool     `json:"is_hypervisor"`    // hosts VMs (role == host)
	IsContainer  bool     `json:"is_container"`     // running inside a container
	Hints        []string `json:"hints,omitempty"`  // extra signals: proxmox, wsl, ...
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "virtualization" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	system, role, err := host.VirtualizationWithContext(ctx)
	if err != nil {
		system, role = "", ""
	}

	stats := Stats{
		System:       system,
		Role:         role,
		IsHypervisor: role == "host",
		IsVirtual:    role == "guest" || isContainerSystem(system),
		IsContainer:  isContainerSystem(system),
		Hints:        extraDetect(),
	}
	return json.Marshal(stats)
}

// isContainerSystem reports whether the detected system is a container runtime
// rather than a full VM.
func isContainerSystem(system string) bool {
	switch system {
	case "docker", "lxc", "podman", "containerd", "openvz", "rkt":
		return true
	}
	return false
}
