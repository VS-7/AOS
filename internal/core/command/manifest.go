package command

import "github.com/google/jsonschema-go/jsonschema"

// Manifest is the whole published surface in one document: every group, every
// command, every input schema.
//
// It exists because the surface is not linked into every process that has to
// describe it. The terminal binary carries four commands — supervision, which
// cannot be delegated to the process being supervised — and asks a daemon for
// the rest, so anything built from that binary's own registry described 4 of
// ~140 commands and called it "the tool surface exactly as it is published"
// (defect #1). This is what the daemon hands over instead.
type Manifest struct {
	Version string          `json:"version,omitempty"`
	Groups  []ManifestGroup `json:"groups"`
}

// ManifestGroup is one group and the commands it publishes.
type ManifestGroup struct {
	Name     string            `json:"name"`
	Tool     string            `json:"tool,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	Doc      string            `json:"doc,omitempty"`
	Hint     string            `json:"hint,omitempty"`
	Commands []ManifestCommand `json:"commands"`
}

// ManifestCommand is one command, described the way each surface needs it: the
// key and summary for a listing, the documentation and schema for a model
// learning the contract, the flags for a client deciding what to offer.
type ManifestCommand struct {
	Key         string             `json:"key"`
	Group       string             `json:"group"`
	Name        string             `json:"name"`
	Summary     string             `json:"summary,omitempty"`
	Doc         string             `json:"doc,omitempty"`
	Aliases     []string           `json:"aliases,omitempty"`
	Examples    []Example          `json:"examples,omitempty"`
	Local       bool               `json:"local"`
	Registry    bool               `json:"agentRegistry"`
	Annotations Annotations        `json:"annotations"`
	InputSchema *jsonschema.Schema `json:"inputSchema,omitempty"`
}

// ManifestOf renders a registry as a manifest, groups and commands in the same
// alphabetical order every other surface publishes them in.
func ManifestOf(r *Registry, version string) Manifest {
	groups := r.Groups()
	out := Manifest{Version: version, Groups: make([]ManifestGroup, 0, len(groups))}
	for _, g := range groups {
		entry := ManifestGroup{
			Name:     g.Name,
			Tool:     g.Tool,
			Summary:  g.Summary,
			Doc:      g.Doc,
			Hint:     g.Hint,
			Commands: make([]ManifestCommand, 0, len(g.Commands)),
		}
		for _, d := range g.Commands {
			entry.Commands = append(entry.Commands, ManifestCommand{
				Key:         d.Key(),
				Group:       d.Group(),
				Name:        d.Name(),
				Summary:     d.Summary(),
				Doc:         d.Doc(),
				Aliases:     d.Aliases(),
				Examples:    d.Examples(),
				Local:       d.Local(),
				Registry:    d.InRegistry(),
				Annotations: d.Annotations(),
				InputSchema: d.InputSchema(),
			})
		}
		out.Groups = append(out.Groups, entry)
	}
	return out
}

// Commands flattens the manifest, in key order within each group.
func (m Manifest) Commands() []ManifestCommand {
	var out []ManifestCommand
	for _, g := range m.Groups {
		out = append(out, g.Commands...)
	}
	return out
}
