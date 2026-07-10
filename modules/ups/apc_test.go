package ups

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

// fakeAPCUPSD starts a minimal apcupsd NIS server that answers one "status"
// request with the given lines, then closes.
func fakeAPCUPSD(t *testing.T, lines []string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer l.Close()

		// Read the length-prefixed request ("status").
		var lb [2]byte
		if _, err := io.ReadFull(conn, lb[:]); err != nil {
			return
		}
		req := make([]byte, binary.BigEndian.Uint16(lb[:]))
		io.ReadFull(conn, req)

		// Respond with framed lines then a zero-length terminator.
		for _, line := range lines {
			frame := make([]byte, 2+len(line))
			binary.BigEndian.PutUint16(frame, uint16(len(line)))
			copy(frame[2:], line)
			conn.Write(frame)
		}
		conn.Write([]byte{0, 0})
	}()
	return l.Addr().String()
}

func TestAPCDriver(t *testing.T) {
	addr := fakeAPCUPSD(t, []string{
		"MODEL    : Back-UPS ES 700\n",
		"STATUS   : ONLINE\n",
		"BCHARGE  : 100.0 Percent\n",
		"TIMELEFT : 45.0 Minutes\n",
		"LINEV    : 123.0 Volts\n",
		"OUTPUTV  : 123.0 Volts\n",
		"LOADPCT  : 12.0 Percent\n",
		"ITEMP    : 30.5 C\n",
		"BATTV    : 27.3 Volts\n",
		"SERIALNO : ABC123\n",
	})
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	d := newAPCDriver(map[string]any{"host": host, "port": port, "name": "test-apc"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	upses, err := d.query(ctx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(upses) != 1 {
		t.Fatalf("expected 1 ups, got %d", len(upses))
	}
	u := upses[0]

	if u.Status != "online" {
		t.Errorf("status = %q, want online", u.Status)
	}
	if u.BatteryCharge != 100 {
		t.Errorf("charge = %v, want 100", u.BatteryCharge)
	}
	if u.BatteryRuntime != 2700 { // 45 minutes -> seconds
		t.Errorf("runtime = %d, want 2700", u.BatteryRuntime)
	}
	if u.InputVoltage != 123 || u.OutputVoltage != 123 {
		t.Errorf("voltage in/out = %v/%v, want 123/123", u.InputVoltage, u.OutputVoltage)
	}
	if u.Load != 12 {
		t.Errorf("load = %v, want 12", u.Load)
	}
	if u.Temperature != 30.5 {
		t.Errorf("temp = %v, want 30.5", u.Temperature)
	}
	if u.Model != "Back-UPS ES 700" || u.Serial != "ABC123" {
		t.Errorf("model/serial = %q/%q", u.Model, u.Serial)
	}
	if u.Manufacturer != "APC" {
		t.Errorf("manufacturer = %q, want APC", u.Manufacturer)
	}
}

func TestMapAPCStatus(t *testing.T) {
	cases := map[string]string{
		"ONLINE":         "online",
		"ONBATT":         "on_battery",
		"ONBATT LOWBATT": "low_battery",
		"":               "unknown",
	}
	for in, want := range cases {
		if got := mapAPCStatus(in); got != want {
			t.Errorf("mapAPCStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
