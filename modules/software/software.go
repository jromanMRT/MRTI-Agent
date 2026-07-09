// Package software reports installed programs and their versions: dpkg/rpm on
// Linux, the Uninstall registry keys on Windows. This feeds asset inventory
// and (later) patch/update tracking.
package software

import (
	"context"
	"encoding/json"

	"github.com/jromanMRT/mrti-agent/modules"
)

func init() { modules.Register("software", func() modules.Module { return &Module{} }) }

// Program is one installed package/application.
type Program struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Publisher string `json:"publisher,omitempty"`
}

// Stats is the software payload.
type Stats struct {
	Source   string    `json:"source"` // dpkg | rpm | registry
	Total    int       `json:"total"`
	Programs []Program `json:"programs"`
}

// Module implements modules.Module.
type Module struct {
	modules.BaseModule
}

func (m *Module) Name() string { return "software" }

func (m *Module) Collect(ctx context.Context) (json.RawMessage, error) {
	source, progs, err := listSoftware(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Stats{Source: source, Total: len(progs), Programs: progs})
}
