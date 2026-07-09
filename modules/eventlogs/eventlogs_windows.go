//go:build windows

package eventlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// collectEvents queries the Windows Event Log via Get-WinEvent, filtering to
// Critical/Error/Warning levels within the window across the configured logs.
func collectEvents(ctx context.Context, since time.Duration, max int, logs []string) (string, []Event, error) {
	if len(logs) == 0 {
		logs = []string{"System", "Application"}
	}
	quoted := make([]string, len(logs))
	for i, l := range logs {
		quoted[i] = "'" + strings.ReplaceAll(l, "'", "''") + "'"
	}
	logList := strings.Join(quoted, ",")

	script := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue';`+
		`$start=(Get-Date).AddSeconds(-%d);`+
		`Get-WinEvent -FilterHashtable @{LogName=@(%s); Level=@(1,2,3); StartTime=$start} -MaxEvents %d -ErrorAction SilentlyContinue |`+
		`Select-Object @{n='time';e={$_.TimeCreated.ToString('o')}},`+
		`@{n='level';e={$_.LevelDisplayName}},`+
		`@{n='source';e={$_.ProviderName}},`+
		`@{n='message';e={$_.Message}} | ConvertTo-Json -Compress -Depth 3`,
		int(since.Seconds()), logList, max)

	ps := powershellBinary()
	out, err := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return "windows-eventlog", nil, nil
	}

	events := parseWinEvents(out)
	return "windows-eventlog", events, nil
}

// parseWinEvents handles ConvertTo-Json's quirk of emitting a bare object for a
// single result versus an array for many.
func parseWinEvents(data []byte) []Event {
	type raw struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Source  string `json:"source"`
		Message string `json:"message"`
	}
	trimmed := strings.TrimSpace(string(data))

	var rows []raw
	if strings.HasPrefix(trimmed, "[") {
		if json.Unmarshal(data, &rows) != nil {
			return nil
		}
	} else {
		var one raw
		if json.Unmarshal(data, &one) != nil {
			return nil
		}
		rows = []raw{one}
	}

	events := make([]Event, 0, len(rows))
	for _, r := range rows {
		t, _ := time.Parse(time.RFC3339, r.Time)
		events = append(events, Event{
			Time:    t,
			Level:   normalizeLevel(r.Level),
			Source:  r.Source,
			Message: r.Message,
		})
	}
	return events
}

func normalizeLevel(l string) string {
	switch strings.ToLower(l) {
	case "critical":
		return "critical"
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "warning"
	}
}

func powershellBinary() string {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh"
	}
	return "powershell"
}
