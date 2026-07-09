//go:build linux

package network

import (
	"bufio"
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"strings"
)

// systemResolvers parses /etc/resolv.conf for configured nameservers.
func systemResolvers() []string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "nameserver ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "nameserver")))
		}
	}
	return out
}

// systemGateway reads /proc/net/route and returns the IPv4 default gateway
// (the route whose destination is 0.0.0.0).
func systemGateway() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "00000000" { // destination must be default route
			continue
		}
		gwHex := fields[2]
		v, err := strconv.ParseUint(gwHex, 16, 32)
		if err != nil {
			continue
		}
		ip := make(net.IP, 4)
		binary.LittleEndian.PutUint32(ip, uint32(v)) // route is little-endian
		return ip.String()
	}
	return ""
}
