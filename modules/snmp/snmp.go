// Package snmp polls remote SNMP devices (switches, routers, APs, printers,
// UPSes, NAS) configured for this agent. Unlike the host collectors, this
// module reaches out over the network to each target, fetches the standard
// system group plus any custom named OIDs, and returns per-device results.
// SNMP v1/v2c are fully supported; v3 (USM) is supported with user/auth/priv.
package snmp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("snmp", func() modules.Module { return &Module{} }) }

// Standard SNMPv2-MIB system group OIDs.
var systemOIDs = map[string]string{
	"sysDescr":    "1.3.6.1.2.1.1.1.0",
	"sysUpTime":   "1.3.6.1.2.1.1.3.0",
	"sysContact":  "1.3.6.1.2.1.1.4.0",
	"sysName":     "1.3.6.1.2.1.1.5.0",
	"sysLocation": "1.3.6.1.2.1.1.6.0",
}

// target is one configured SNMP device.
type target struct {
	Name           string            `json:"name"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Version        string            `json:"version"` // 1 | 2c | 3
	Community      string            `json:"community"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	OIDs           map[string]string `json:"oids"`
	// SNMPv3 (USM)
	Username      string `json:"username"`
	SecurityLevel string `json:"security_level"` // noAuthNoPriv | authNoPriv | authPriv
	AuthProtocol  string `json:"auth_protocol"`  // MD5 | SHA
	AuthPassword  string `json:"auth_password"`
	PrivProtocol  string `json:"priv_protocol"` // DES | AES
	PrivPassword  string `json:"priv_password"`
}

// DeviceResult is the poll outcome for one target.
type DeviceResult struct {
	Name      string            `json:"name"`
	Host      string            `json:"host"`
	Reachable bool              `json:"reachable"`
	System    map[string]string `json:"system,omitempty"`
	Values    map[string]string `json:"values,omitempty"`
	LatencyMS float64           `json:"latency_ms,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// Stats is the snmp payload.
type Stats struct {
	Devices []DeviceResult `json:"devices"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
	targets []target
}

func (m *Module) Name() string { return "snmp" }

// Configure parses the target list. Nested config is re-marshalled through JSON
// for clean struct decoding.
func (m *Module) Configure(settings map[string]any, log *slog.Logger) error {
	m.BaseModule.Configure(settings, log)
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	var cfg struct {
		Targets []target `json:"targets"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	m.targets = cfg.Targets
	return nil
}

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	results := make([]DeviceResult, 0, len(m.targets))
	for _, t := range m.targets {
		results = append(results, m.poll(ctx, t))
	}
	return json.Marshal(Stats{Devices: results})
}

// poll queries a single target and returns its result. Network/agent errors
// are captured per-device, never aborting the cycle.
func (m *Module) poll(_ context.Context, t target) DeviceResult {
	res := DeviceResult{Name: t.Name, Host: t.Host}

	client, err := buildClient(t)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	start := time.Now()
	if err := client.Connect(); err != nil {
		res.Error = "connect: " + err.Error()
		return res
	}
	defer client.Conn.Close()

	// Assemble the OID list: standard system group + custom named OIDs.
	names := make([]string, 0, len(systemOIDs)+len(t.OIDs))
	oids := make([]string, 0, len(systemOIDs)+len(t.OIDs))
	for n, o := range systemOIDs {
		names = append(names, n)
		oids = append(oids, o)
	}
	for n, o := range t.OIDs {
		names = append(names, n)
		oids = append(oids, o)
	}

	packet, err := client.Get(oids)
	if err != nil {
		res.Error = "get: " + err.Error()
		return res
	}
	res.Reachable = true
	res.LatencyMS = float64(time.Since(start).Microseconds()) / 1000.0
	res.System = map[string]string{}
	res.Values = map[string]string{}

	for i, v := range packet.Variables {
		if i >= len(names) {
			break
		}
		name := names[i]
		val := valueToString(v)
		if _, isSystem := systemOIDs[name]; isSystem {
			res.System[name] = val
		} else {
			res.Values[name] = val
		}
	}
	return res
}

// buildClient constructs a gosnmp client for the target's version and creds.
func buildClient(t target) (*gosnmp.GoSNMP, error) {
	port := t.Port
	if port == 0 {
		port = 161
	}
	timeout := time.Duration(t.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	g := &gosnmp.GoSNMP{
		Target:  t.Host,
		Port:    uint16(port),
		Timeout: timeout,
		Retries: 1,
	}

	switch t.Version {
	case "1":
		g.Version = gosnmp.Version1
		g.Community = orDefault(t.Community, "public")
	case "", "2c", "2":
		g.Version = gosnmp.Version2c
		g.Community = orDefault(t.Community, "public")
	case "3":
		g.Version = gosnmp.Version3
		if err := applyV3(g, t); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported snmp version %q", t.Version)
	}
	return g, nil
}

func applyV3(g *gosnmp.GoSNMP, t target) error {
	g.SecurityModel = gosnmp.UserSecurityModel
	usm := &gosnmp.UsmSecurityParameters{UserName: t.Username}

	switch t.SecurityLevel {
	case "noAuthNoPriv", "":
		g.MsgFlags = gosnmp.NoAuthNoPriv
	case "authNoPriv":
		g.MsgFlags = gosnmp.AuthNoPriv
	case "authPriv":
		g.MsgFlags = gosnmp.AuthPriv
	default:
		return fmt.Errorf("unsupported snmp v3 security_level %q", t.SecurityLevel)
	}

	if g.MsgFlags != gosnmp.NoAuthNoPriv {
		switch t.AuthProtocol {
		case "SHA", "sha":
			usm.AuthenticationProtocol = gosnmp.SHA
		case "MD5", "md5", "":
			usm.AuthenticationProtocol = gosnmp.MD5
		default:
			return fmt.Errorf("unsupported auth_protocol %q", t.AuthProtocol)
		}
		usm.AuthenticationPassphrase = t.AuthPassword
	}
	if g.MsgFlags == gosnmp.AuthPriv {
		switch t.PrivProtocol {
		case "AES", "aes":
			usm.PrivacyProtocol = gosnmp.AES
		case "DES", "des", "":
			usm.PrivacyProtocol = gosnmp.DES
		default:
			return fmt.Errorf("unsupported priv_protocol %q", t.PrivProtocol)
		}
		usm.PrivacyPassphrase = t.PrivPassword
	}
	g.SecurityParameters = usm
	return nil
}

// valueToString renders an SNMP PDU value as a readable string.
func valueToString(v gosnmp.SnmpPDU) string {
	switch v.Type {
	case gosnmp.OctetString:
		if b, ok := v.Value.([]byte); ok {
			return string(b)
		}
	case gosnmp.ObjectIdentifier:
		if s, ok := v.Value.(string); ok {
			return s
		}
	case gosnmp.TimeTicks:
		return fmt.Sprintf("%v", v.Value)
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance:
		return ""
	}
	return fmt.Sprintf("%v", v.Value)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
