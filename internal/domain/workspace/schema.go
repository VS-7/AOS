package workspace

import (
	"strings"

	"github.com/OWNER/aos/internal/core/command"
)

// ListInput is the payload of `workspace list`.
type ListInput struct {
	IncludeArchived bool `json:"includeArchived,omitempty" jsonschema:"Include archived workspaces in the result. Off by default."`

	command.Reasoning
}

// ListOutput is what `workspace list` returns.
type ListOutput struct {
	Workspaces []Workspace `json:"workspaces" jsonschema:"The registered workspaces, ordered by id."`
	Total      int         `json:"total" jsonschema:"How many workspaces matched."`
}

// GetInput is the payload of `workspace get`.
//
// Two names for one identifier. `workspace` is what this group has always
// published and what every existing caller sends; `id` is what the other
// twelve groups call the identifier of their own resource, so a caller
// building calls by convention — a generated form, an agent that has learned
// the shape of the rest of the surface — is right too (defect #7).
type GetInput struct {
	Workspace string `json:"workspace,omitempty" cli:"arg" jsonschema:"Workspace id. Omit to read the active one, taken from the environment."`
	ID        string `json:"id,omitempty" jsonschema:"Workspace id. The same thing as workspace, under the name every other group uses."`

	command.Reasoning
}

// Target is the workspace this payload addresses, under either name.
func (in GetInput) Target() string { return firstNamed(in.Workspace, in.ID) }

// CreateInput registers a new workspace.
type CreateInput struct {
	Name         string            `json:"name" cli:"arg" jsonschema:"Workspace name. The identifier is the slug of this name. Example: \"Project Alpha\"." validate:"required,notblank"`
	Path         string            `json:"path,omitempty" jsonschema:"Absolute path of the repository this workspace operates on. Omit to create one under the state directory."`
	Logo         string            `json:"logo,omitempty" jsonschema:"Optional logo URL."`
	Color        string            `json:"color,omitempty" jsonschema:"Optional accent colour as a hex code. Example: \"#5E6AD2\"."`
	Orchestrator *OrchestratorSpec `json:"orchestrator,omitempty" jsonschema:"Tone, style and autonomy of the orchestrator agent created with the workspace."`

	command.Reasoning
}

// CreateOutput reports what creating a workspace actually did.
//
// It carries more than the record because the operation is more than a write:
// it lays out directories in a repository the user owns, may put that
// repository under version control, and creates the first agent. A caller that
// only received the Workspace would have no way to know which of those
// happened.
type CreateOutput struct {
	Workspace    Workspace      `json:"workspace" jsonschema:"The registered workspace."`
	Orchestrator string         `json:"orchestrator,omitempty" jsonschema:"Slug of the orchestrator agent that owns this workspace."`
	Scaffold     ScaffoldReport `json:"scaffold" jsonschema:"What the scaffolding step created."`
	Adopted      bool           `json:"adopted" jsonschema:"True when an existing layout was adopted rather than created."`
}

// UpdateInput patches a workspace. Fields are addressed by dotted path, as in
// the configuration commands, so that a caller can change one nested value
// without sending the whole record back and racing another writer.
type UpdateInput struct {
	Workspace string         `json:"workspace,omitempty" cli:"arg" jsonschema:"Workspace id. Omit to update the active one — the one this session is scoped to."`
	ID        string         `json:"id,omitempty" jsonschema:"Workspace id. The same thing as workspace, under the name every other group uses."`
	Set       map[string]any `json:"set" jsonschema:"Fields to change, addressed by dotted path. Example: { \"git.branchPrefix\": \"feat\", \"color\": \"#10b981\" }." validate:"required,min=1"`

	command.Reasoning
}

// Target is the workspace this payload addresses, under either name.
func (in UpdateInput) Target() string { return firstNamed(in.Workspace, in.ID) }

// DeleteInput removes a workspace from the registry.
//
// Either name identifies it, but one of the two is required: unregistering
// resolves nothing implicitly, because "the active one" is not something to
// guess at when the answer is destructive.
type DeleteInput struct {
	Workspace string `json:"workspace,omitempty" cli:"arg" jsonschema:"Workspace id to unregister."`
	ID        string `json:"id,omitempty" jsonschema:"Workspace id to unregister. The same thing as workspace, under the name every other group uses."`

	command.Reasoning
}

// Target is the workspace this payload addresses, under either name.
func (in DeleteInput) Target() string { return firstNamed(in.Workspace, in.ID) }

// DeleteOutput reports what was unregistered, and what was deliberately left
// behind.
type DeleteOutput struct {
	Workspace string `json:"workspace" jsonschema:"The workspace that was unregistered."`
	Deleted   bool   `json:"deleted" jsonschema:"True when the workspace existed and was unregistered."`
	Path      string `json:"path,omitempty" jsonschema:"The repository that stays on disk, untouched."`
}

// IntrospectInput registers the repository the caller is standing in.
type IntrospectInput struct {
	Path string `json:"path,omitempty" jsonschema:"Repository to register. Defaults to the working directory."`

	command.Reasoning
}

// InventoryInput asks what a workspace holds.
type InventoryInput struct {
	Workspace string `json:"workspace,omitempty" cli:"arg" jsonschema:"Workspace id. Omit to survey the active one."`
	ID        string `json:"id,omitempty" jsonschema:"Workspace id. The same thing as workspace, under the name every other group uses."`

	command.Reasoning
}

// Target is the workspace this payload addresses, under either name.
func (in InventoryInput) Target() string { return firstNamed(in.Workspace, in.ID) }

// firstNamed picks whichever of the two spellings of the identifier the caller
// used. `workspace` wins when both are sent, because it is the one this group
// published first.
func firstNamed(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// Inventory is the panoramic view an agent reads to orient itself: what exists
// here, of what kind, how much of it.
//
// It reports names and counts, never bodies. An inventory that inlined content
// would be the most expensive call in the system and would be made at the start
// of every session.
type Inventory struct {
	Workspace   string              `json:"workspace" jsonschema:"The workspace surveyed."`
	Name        string              `json:"name" jsonschema:"Its human-readable name."`
	Path        string              `json:"path" jsonschema:"Its repository root."`
	Collections []CollectionSummary `json:"collections" jsonschema:"What the workspace holds, by collection."`
	TaskTypes   []TaskType          `json:"taskTypes" jsonschema:"The task taxonomy in force here."`
	Total       int                 `json:"total" jsonschema:"Total number of records across every collection."`
}

// CollectionSummary is one line of the inventory.
type CollectionSummary struct {
	Name  string   `json:"name" jsonschema:"Collection name. Example: \"memories\"."`
	Count int      `json:"count" jsonschema:"How many records it holds."`
	Keys  []string `json:"keys,omitempty" jsonschema:"Identifiers of the records, when there are few enough to list."`
}
