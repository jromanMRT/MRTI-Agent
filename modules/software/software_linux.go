//go:build linux

package software

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// listSoftware prefers dpkg (Debian/Ubuntu) and falls back to rpm
// (RHEL/Fedora/SUSE). Returns the source used so the Core knows the origin.
func listSoftware(ctx context.Context) (string, []Program, error) {
	if _, err := exec.LookPath("dpkg-query"); err == nil {
		progs, err := dpkg(ctx)
		return "dpkg", progs, err
	}
	if _, err := exec.LookPath("rpm"); err == nil {
		progs, err := rpm(ctx)
		return "rpm", progs, err
	}
	return "unknown", nil, nil
}

func dpkg(ctx context.Context) ([]Program, error) {
	out, err := exec.CommandContext(ctx, "dpkg-query", "-W",
		"-f=${Package}\t${Version}\t${Maintainer}\n").Output()
	if err != nil {
		return nil, err
	}
	return parseTabbed(out), nil
}

func rpm(ctx context.Context) ([]Program, error) {
	out, err := exec.CommandContext(ctx, "rpm", "-qa",
		"--qf", "%{NAME}\t%{VERSION}-%{RELEASE}\t%{VENDOR}\n").Output()
	if err != nil {
		return nil, err
	}
	return parseTabbed(out), nil
}

func parseTabbed(data []byte) []Program {
	var progs []Program
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.SplitN(sc.Text(), "\t", 3)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		p := Program{Name: fields[0]}
		if len(fields) > 1 {
			p.Version = fields[1]
		}
		if len(fields) > 2 {
			p.Publisher = fields[2]
		}
		progs = append(progs, p)
	}
	return progs
}
