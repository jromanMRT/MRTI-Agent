//go:build windows

package software

import (
	"context"

	"golang.org/x/sys/windows/registry"
)

// uninstallRoots are the registry locations Windows records installed programs
// under, covering 64-bit, 32-bit-on-64 (WOW6432Node) and per-user installs.
var uninstallRoots = []struct {
	Key  registry.Key
	Path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// listSoftware enumerates the Uninstall registry keys and de-duplicates by
// name+version. Entries without a DisplayName (updates, components) are skipped.
func listSoftware(_ context.Context) (string, []Program, error) {
	seen := map[string]struct{}{}
	var progs []Program

	for _, root := range uninstallRoots {
		k, err := registry.OpenKey(root.Key, root.Path, registry.READ)
		if err != nil {
			continue
		}
		names, err := k.ReadSubKeyNames(-1)
		if err != nil {
			k.Close()
			continue
		}
		for _, name := range names {
			sub, err := registry.OpenKey(root.Key, root.Path+`\`+name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			display, _, err := sub.GetStringValue("DisplayName")
			if err != nil || display == "" {
				sub.Close()
				continue
			}
			version, _, _ := sub.GetStringValue("DisplayVersion")
			publisher, _, _ := sub.GetStringValue("Publisher")
			sub.Close()

			key := display + "\x00" + version
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			progs = append(progs, Program{Name: display, Version: version, Publisher: publisher})
		}
		k.Close()
	}
	return "registry", progs, nil
}
