package ups

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// apcDriver speaks the apcupsd NIS protocol (TCP, default port 3551). The
// client sends a length-prefixed "status" request; the daemon replies with a
// series of length-prefixed "KEY : value" lines terminated by a zero-length
// frame. This is a native implementation — no apcaccess binary required.
type apcDriver struct {
	host string
	port int
	name string
}

func newAPCDriver(settings map[string]any) *apcDriver {
	d := &apcDriver{host: "127.0.0.1", port: 3551}
	if v, ok := settings["host"].(string); ok && v != "" {
		d.host = v
	}
	if v, ok := settings["port"]; ok {
		if n, ok := toInt(v); ok && n > 0 {
			d.port = n
		}
	}
	if v, ok := settings["name"].(string); ok {
		d.name = v
	}
	return d
}

func (d *apcDriver) query(ctx context.Context) ([]UPSInfo, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(d.host, strconv.Itoa(d.port)))
	if err != nil {
		return nil, fmt.Errorf("connect apcupsd %s:%d: %w", d.host, d.port, err)
	}
	defer conn.Close()

	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	} else {
		conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	if err := writeAPC(conn, "status"); err != nil {
		return nil, err
	}
	vars, err := readAPC(conn)
	if err != nil {
		return nil, err
	}

	name := d.name
	if name == "" {
		if u := vars["UPSNAME"]; u != "" {
			name = u
		} else {
			name = "apc"
		}
	}
	return []UPSInfo{buildAPCInfo(name, vars)}, nil
}

// writeAPC sends a length-prefixed command.
func writeAPC(w io.Writer, cmd string) error {
	buf := make([]byte, 2+len(cmd))
	binary.BigEndian.PutUint16(buf, uint16(len(cmd)))
	copy(buf[2:], cmd)
	_, err := w.Write(buf)
	return err
}

// readAPC reads length-prefixed frames until the terminating zero-length frame,
// parsing each "KEY : value" line into a map keyed by the trimmed KEY.
func readAPC(r io.Reader) (map[string]string, error) {
	br := bufio.NewReader(r)
	vars := map[string]string{}
	var lenBuf [2]byte
	for {
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			return nil, err
		}
		n := binary.BigEndian.Uint16(lenBuf[:])
		if n == 0 {
			return vars, nil
		}
		line := make([]byte, n)
		if _, err := io.ReadFull(br, line); err != nil {
			return nil, err
		}
		k, v, ok := parseAPCLine(string(line))
		if ok {
			vars[k] = v
		}
	}
}

// parseAPCLine splits "BCHARGE  : 100.0 Percent" into ("BCHARGE", "100.0 Percent").
func parseAPCLine(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:i])
	val := strings.TrimSpace(strings.TrimRight(line[i+1:], "\n"))
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

// buildAPCInfo maps apcupsd status variables to the normalised UPSInfo. Values
// carry units (e.g. "100.0 Percent", "27.5 Minutes"), so we take the leading
// number.
func buildAPCInfo(name string, vars map[string]string) UPSInfo {
	info := UPSInfo{
		Name:           name,
		Manufacturer:   "APC",
		Model:          vars["MODEL"],
		Serial:         vars["SERIALNO"],
		StatusRaw:      vars["STATUS"],
		Status:         mapAPCStatus(vars["STATUS"]),
		BatteryCharge:  num(vars["BCHARGE"]),
		BatteryRuntime: int(num(vars["TIMELEFT"]) * 60), // TIMELEFT is minutes
		BatteryVoltage: num(vars["BATTV"]),
		InputVoltage:   num(vars["LINEV"]),
		OutputVoltage:  num(vars["OUTPUTV"]),
		Load:           num(vars["LOADPCT"]),
		Temperature:    num(vars["ITEMP"]),
	}
	return info
}

// mapAPCStatus translates apcupsd STATUS flags to the common vocabulary.
func mapAPCStatus(raw string) string {
	u := strings.ToUpper(raw)
	switch {
	case strings.Contains(u, "LOWBATT"):
		return "low_battery"
	case strings.Contains(u, "ONBATT"):
		return "on_battery"
	case strings.Contains(u, "ONLINE"):
		return "online"
	case raw == "":
		return "unknown"
	default:
		return strings.ToLower(strings.Fields(raw)[0])
	}
}

// num extracts the leading float from a value like "126.0 Volts".
func num(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
