//go:build linux

package eventlogs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"time"
)

// collectEvents reads the systemd journal for warning-and-above entries within
// the window, via `journalctl -o json`. The logs argument is unused on Linux.
func collectEvents(ctx context.Context, since time.Duration, max int, _ []string) (string, []Event, error) {
	sinceArg := time.Now().Add(-since).Format("2006-01-02 15:04:05")
	args := []string{
		"--no-pager", "-o", "json",
		"-p", "warning", // warning (4) and more severe
		"--since", sinceArg,
		"-n", strconv.Itoa(max),
	}
	out, err := exec.CommandContext(ctx, "journalctl", args...).Output()
	if err != nil {
		// journalctl missing or unreadable — treat as empty, not fatal.
		return "journald", nil, nil
	}

	var events []Event
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var row map[string]json.RawMessage
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		events = append(events, Event{
			Time:    journalTime(row["__REALTIME_TIMESTAMP"]),
			Level:   priorityLevel(jstr(row["PRIORITY"])),
			Source:  jstr(row["SYSLOG_IDENTIFIER"]),
			Unit:    jstr(row["_SYSTEMD_UNIT"]),
			Message: jstr(row["MESSAGE"]),
		})
	}
	return "journald", events, nil
}

// jstr extracts a JSON string; journald sometimes encodes non-UTF8 MESSAGE as
// an array of byte numbers, which we render best-effort.
func jstr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var bnums []byte
	if json.Unmarshal(raw, &bnums) == nil {
		return string(bnums)
	}
	return ""
}

func journalTime(raw json.RawMessage) time.Time {
	s := jstr(raw)
	if s == "" {
		return time.Now()
	}
	usec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(0, usec*1000)
}

// priorityLevel maps a syslog priority number to the common vocabulary.
func priorityLevel(p string) string {
	switch p {
	case "0", "1", "2":
		return "critical"
	case "3":
		return "error"
	case "4":
		return "warning"
	default:
		return "warning"
	}
}
