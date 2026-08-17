package file

import "time"

// Node is one entry in a workspace file tree.
type Node struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Dir        bool      `json:"dir"`
	Size       int64     `json:"size"`
	Extension  string    `json:"extension,omitempty"`
	MediaType  string    `json:"mediaType,omitempty"`
	Editable   bool      `json:"editable"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// Tree is a workspace-relative listing rooted at the path a caller asked for.
type Tree struct {
	Path  string `json:"path"`
	Nodes []Node `json:"nodes"`
}

// Content is a file's body. It carries Text for anything the UI can edit and
// Base64 for everything else — never both, so a caller never has to guess
// which field actually holds the file.
type Content struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Text      string `json:"text,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
}

// Diff is one file's change against HEAD, powering the task worktree review
// screen.
type Diff struct {
	Path     string  `json:"path"`
	Status   string  `json:"status"` // added, modified, deleted, untracked, unchanged
	IsBinary bool    `json:"isBinary"`
	OldText  *string `json:"oldText,omitempty"`
	NewText  *string `json:"newText,omitempty"`
}
