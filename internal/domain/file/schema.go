package file

// TreeInput selects the directory a Tree call lists.
//
// There is no command.Reasoning field here on purpose: this feature has no
// command group and no MCP tools ([[File]] in the vault) — the agent reaches
// the filesystem through its sandbox, never through this API, so nothing
// here is a tool surface that owes an explanation.
type TreeInput struct {
	Path      string `json:"path,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
}

// ReadInput selects the file a Read call returns the content of.
type ReadInput struct {
	Path string `json:"path"`
}

// WriteInput is the content a Write call persists. A path with nothing at it
// yet is created, parent directories included — this interface has no
// separate create, so write is how a new file comes to exist.
type WriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// DiffInput selects the file a Diff call compares against HEAD.
type DiffInput struct {
	Path string `json:"path"`
}
