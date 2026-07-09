// Package network collects per-interface addressing and traffic counters plus
// host-level DNS and default gateway information.
package network

import (
	"context"
	"encoding/json"
	"net"

	"github.com/jromanMRT/mrti-agent/modules"
	psnet "github.com/shirou/gopsutil/v4/net"
)

func init() { modules.Register("network", func() modules.Module { return &Module{} }) }

// Interface describes one network interface with its addresses and counters.
type Interface struct {
	Name        string   `json:"name"`
	MAC         string   `json:"mac,omitempty"`
	Addrs       []string `json:"addrs,omitempty"`
	MTU         int      `json:"mtu,omitempty"`
	Up          bool     `json:"up"`
	BytesSent   uint64   `json:"bytes_sent"`
	BytesRecv   uint64   `json:"bytes_recv"`
	PacketsSent uint64   `json:"packets_sent"`
	PacketsRecv uint64   `json:"packets_recv"`
	ErrIn       uint64   `json:"err_in"`
	ErrOut      uint64   `json:"err_out"`
	DropIn      uint64   `json:"drop_in"`
	DropOut     uint64   `json:"drop_out"`
}

// Stats is the network payload.
type Stats struct {
	Interfaces []Interface `json:"interfaces"`
	DNS        []string    `json:"dns,omitempty"`
	Gateway    string      `json:"gateway,omitempty"`
}

type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "network" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	var s Stats

	// Per-interface counters keyed by name for a quick join with addressing.
	counters := map[string]psnet.IOCountersStat{}
	if io, err := psnet.IOCountersWithContext(ctx, true); err == nil {
		for _, c := range io {
			counters[c.Name] = c
		}
	}

	ifaces, err := psnet.InterfacesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	for _, in := range ifaces {
		iface := Interface{
			Name: in.Name,
			MAC:  in.HardwareAddr,
			MTU:  in.MTU,
			Up:   hasFlag(in.Flags, "up"),
		}
		for _, a := range in.Addrs {
			iface.Addrs = append(iface.Addrs, a.Addr)
		}
		if c, ok := counters[in.Name]; ok {
			iface.BytesSent = c.BytesSent
			iface.BytesRecv = c.BytesRecv
			iface.PacketsSent = c.PacketsSent
			iface.PacketsRecv = c.PacketsRecv
			iface.ErrIn = c.Errin
			iface.ErrOut = c.Errout
			iface.DropIn = c.Dropin
			iface.DropOut = c.Dropout
		}
		s.Interfaces = append(s.Interfaces, iface)
	}

	s.DNS = resolvers()
	s.Gateway = defaultGateway()
	return json.Marshal(s)
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// resolvers returns configured DNS servers. Uses net.DefaultResolver hints on
// all platforms via a lookup of the system config where available.
func resolvers() []string {
	// net does not expose configured servers portably; return nil when unknown.
	// Platform-specific helpers can populate this later (resolv.conf / registry).
	return systemResolvers()
}

// defaultGateway returns the default route's gateway IP, best effort.
func defaultGateway() string {
	return systemGateway()
}

// ip helper kept for potential future validation of addresses.
var _ = net.ParseIP
