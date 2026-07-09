package ups

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// nutDriver speaks the NUT (Network UPS Tools) upsd protocol directly over TCP.
// The protocol is line-based and simple: LIST UPS enumerates devices and
// LIST VAR <ups> dumps a device's variables as `VAR <ups> <key> "<value>"`.
type nutDriver struct {
	host       string
	port       int
	username   string
	password   string
	upsName    string // limit to a single UPS if set
	includeRaw bool
}

func newNUTDriver(settings map[string]any) *nutDriver {
	d := &nutDriver{host: "127.0.0.1", port: 3493}
	if v, ok := settings["host"].(string); ok && v != "" {
		d.host = v
	}
	if v, ok := settings["port"]; ok {
		if n, ok := toInt(v); ok && n > 0 {
			d.port = n
		}
	}
	if v, ok := settings["username"].(string); ok {
		d.username = v
	}
	if v, ok := settings["password"].(string); ok {
		d.password = v
	}
	if v, ok := settings["name"].(string); ok {
		d.upsName = v
	}
	if v, ok := settings["include_raw"].(bool); ok {
		d.includeRaw = v
	}
	return d
}

func (d *nutDriver) query(ctx context.Context) ([]UPSInfo, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(d.host, strconv.Itoa(d.port)))
	if err != nil {
		return nil, fmt.Errorf("connect upsd %s:%d: %w", d.host, d.port, err)
	}
	defer conn.Close()

	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	} else {
		conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	c := &nutConn{r: bufio.NewReader(conn), w: conn}

	// Optional authentication (needed only for privileged operations; reads
	// usually work anonymously, so failures here are non-fatal).
	if d.username != "" {
		_ = c.command("USERNAME " + d.username)
		if d.password != "" {
			_ = c.command("PASSWORD " + d.password)
		}
	}

	names, err := c.listUPS()
	if err != nil {
		return nil, err
	}
	if d.upsName != "" {
		names = []string{d.upsName}
	}

	var out []UPSInfo
	for _, name := range names {
		vars, err := c.listVars(name)
		if err != nil {
			continue
		}
		out = append(out, buildUPSInfo(name, vars, d.includeRaw))
	}
	_ = c.write("LOGOUT")
	return out, nil
}

// nutConn wraps the connection with line-protocol helpers.
type nutConn struct {
	r *bufio.Reader
	w net.Conn
}

func (c *nutConn) write(cmd string) error {
	_, err := c.w.Write([]byte(cmd + "\n"))
	return err
}

// command sends a command and reads the single-line OK/ERR response.
func (c *nutConn) command(cmd string) error {
	if err := c.write(cmd); err != nil {
		return err
	}
	line, err := c.r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.HasPrefix(line, "ERR") {
		return fmt.Errorf("upsd: %s", strings.TrimSpace(line))
	}
	return nil
}

// listUPS returns the configured UPS names.
func (c *nutConn) listUPS() ([]string, error) {
	if err := c.write("LIST UPS"); err != nil {
		return nil, err
	}
	var names []string
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "END LIST UPS":
			return names, nil
		case strings.HasPrefix(line, "ERR"):
			return nil, fmt.Errorf("upsd: %s", line)
		case strings.HasPrefix(line, "UPS "):
			// Format: UPS <name> "<description>"
			fields := strings.SplitN(line, " ", 3)
			if len(fields) >= 2 {
				names = append(names, fields[1])
			}
		}
	}
}

// listVars dumps a UPS's variables as a key/value map.
func (c *nutConn) listVars(name string) (map[string]string, error) {
	if err := c.write("LIST VAR " + name); err != nil {
		return nil, err
	}
	vars := map[string]string{}
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "END LIST VAR"):
			return vars, nil
		case strings.HasPrefix(line, "ERR"):
			return nil, fmt.Errorf("upsd: %s", line)
		case strings.HasPrefix(line, "VAR "):
			// Format: VAR <ups> <key> "<value>"
			key, value := parseVarLine(line)
			if key != "" {
				vars[key] = value
			}
		}
	}
}

// parseVarLine extracts key and quoted value from a `VAR <ups> <key> "<val>"`.
func parseVarLine(line string) (string, string) {
	rest := strings.TrimPrefix(line, "VAR ")
	// Drop the UPS name token.
	sp := strings.IndexByte(rest, ' ')
	if sp < 0 {
		return "", ""
	}
	rest = rest[sp+1:]
	// key then quoted value
	sp = strings.IndexByte(rest, ' ')
	if sp < 0 {
		return "", ""
	}
	key := rest[:sp]
	value := strings.TrimSpace(rest[sp+1:])
	value = strings.Trim(value, "\"")
	return key, value
}

// buildUPSInfo maps standard NUT variable names to the normalised struct.
func buildUPSInfo(name string, vars map[string]string, includeRaw bool) UPSInfo {
	info := UPSInfo{
		Name:           name,
		Manufacturer:   vars["device.mfr"],
		Model:          vars["device.model"],
		Serial:         vars["device.serial"],
		StatusRaw:      vars["ups.status"],
		Status:         mapNUTStatus(vars["ups.status"]),
		BatteryCharge:  f(vars["battery.charge"]),
		BatteryRuntime: int(f(vars["battery.runtime"])),
		BatteryVoltage: f(vars["battery.voltage"]),
		InputVoltage:   f(vars["input.voltage"]),
		OutputVoltage:  f(vars["output.voltage"]),
		Load:           f(vars["ups.load"]),
		Temperature:    f(vars["ups.temperature"]),
	}
	if info.Manufacturer == "" {
		info.Manufacturer = vars["ups.mfr"]
	}
	if info.Model == "" {
		info.Model = vars["ups.model"]
	}
	if includeRaw {
		info.Raw = vars
	}
	return info
}

// mapNUTStatus translates NUT status flags to the common vocabulary. NUT status
// is a space-separated set of flags, e.g. "OL CHRG", "OB LB".
func mapNUTStatus(raw string) string {
	flags := strings.Fields(raw)
	has := func(f string) bool {
		for _, x := range flags {
			if x == f {
				return true
			}
		}
		return false
	}
	switch {
	case has("LB"):
		return "low_battery"
	case has("OB"):
		return "on_battery"
	case has("OL") && has("CHRG"):
		return "charging"
	case has("OL"):
		return "online"
	default:
		if raw == "" {
			return "unknown"
		}
		return "online"
	}
}

func f(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
