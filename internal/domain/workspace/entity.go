// Package workspace is the operational boundary of the system.
//
// Every record belongs to exactly one workspace, and one process serves many.
// The split that makes that work is inherited from the original: the metadata
// of a workspace is centralised under the state directory, while its data lives
// inside the user's own repository, under .aos/. That is what lets the state of
// an agent be committed next to the code it works on.
package workspace

import "time"

// Workspace is the record kept at ~/.aos/workspaces/{id}/config.json.
//
// It is JSON rather than Markdown because it has no body: there is no prose to
// write under the front matter of a workspace, and the original agrees.
type Workspace struct {
	ID   string `json:"id" jsonschema:"Workspace identifier (slug derived from the name). Example: \"project-alpha\"."`
	Name string `json:"name" jsonschema:"Human-readable workspace name. Example: \"Project Alpha\"."`
	Path string `json:"path" jsonschema:"Absolute path of the repository this workspace operates on."`

	Logo  string `json:"logo,omitempty" jsonschema:"Optional logo URL shown in the desktop app."`
	Color string `json:"color,omitempty" jsonschema:"Optional accent colour as a hex code. Example: \"#5E6AD2\"."`

	Tasks  []TaskType `json:"tasks" jsonschema:"Task-type taxonomy of this workspace."`
	Labels []Label    `json:"labels" jsonschema:"Custom labels available to tasks and projects."`

	Worktrees WorktreeOptions `json:"worktrees" jsonschema:"Git worktree policy used when a task is started in isolation."`
	Git       GitOptions      `json:"git" jsonschema:"Git policy: branch naming, push rules and prompt guidance."`

	// Members and Domains exist on disk in the original but are absent from its
	// schema — a leftover of the multi-user migration. Declared here from the
	// start, so the shape on disk and the shape in the type never disagree.
	Members []Member          `json:"members,omitempty" jsonschema:"Workspace membership entries."`
	Domains map[string]Domain `json:"domains,omitempty" jsonschema:"Custom domain routing, keyed by fully-qualified domain name."`

	Archived bool `json:"archived" jsonschema:"Whether the workspace is archived and hidden from the default listing."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the workspace was registered."`
	UpdatedAt time.Time `json:"updatedAt" jsonschema:"When the workspace record last changed."`
}

// TaskType is one entry of the workspace's task taxonomy.
//
// Instructions is the field that makes the taxonomy more than colour-coding: a
// task of type "bug" injects different guidance into the prompt than one of
// type "docs".
type TaskType struct {
	ID           string `json:"id" jsonschema:"Task type identifier. Example: \"bug\"."`
	Label        string `json:"label" jsonschema:"Human-readable label. Example: \"Bug\"."`
	Color        string `json:"color" jsonschema:"Hex colour for the type. Example: \"#ef4444\"."`
	Description  string `json:"description,omitempty" jsonschema:"What this type of work means in this workspace."`
	Instructions string `json:"instructions,omitempty" jsonschema:"Agent guidance injected for tasks of this type. Example: \"Reproduce and isolate before fixing.\"."`
}

// Label is a user-defined tag available across the workspace.
type Label struct {
	ID    string `json:"id" jsonschema:"Label identifier. Example: \"ui-ux\"."`
	Label string `json:"label" jsonschema:"Human-readable label. Example: \"UI/UX\"."`
	Icon  string `json:"icon,omitempty" jsonschema:"Icon name used by the desktop app. Example: \"Palette\"."`
	Color string `json:"color,omitempty" jsonschema:"Hex colour for the label. Example: \"#ec4899\"."`
}

// Member binds a user account to a workspace.
type Member struct {
	UserID  string    `json:"userId" jsonschema:"User identifier of the member."`
	Role    string    `json:"role" jsonschema:"Membership role: owner or member."`
	AddedAt time.Time `json:"addedAt" jsonschema:"When the member was added."`
}

// Domain maps a fully-qualified domain name to a published artifact.
type Domain struct {
	ArtifactID  string    `json:"artifactId" jsonschema:"Artifact this domain resolves to."`
	WorkspaceID string    `json:"workspaceId" jsonschema:"Workspace the artifact belongs to."`
	CreatedAt   time.Time `json:"createdAt" jsonschema:"When the domain was registered."`
}

// WorktreeOptions is the isolation policy for autonomous task execution.
type WorktreeOptions struct {
	DeleteOldWorktrees bool   `json:"deleteOldWorktrees" jsonschema:"Remove stale worktrees automatically."`
	WorktreeLimit      int    `json:"worktreeLimit" jsonschema:"Maximum number of concurrent worktrees. Between 1 and 50."`
	OnCreateScript     string `json:"onCreateScript,omitempty" jsonschema:"Shell script run after a worktree is created. Example: \"go mod download\"."`
}

// GitOptions is the version-control policy of the workspace.
type GitOptions struct {
	BranchPrefix       string `json:"branchPrefix,omitempty" jsonschema:"Prefix for generated branch names. Example: \"aos\"."`
	ForcePush          bool   `json:"forcePush" jsonschema:"Allow force-pushing to shared branches."`
	CommitInstructions string `json:"commitInstructions,omitempty" jsonschema:"Guidance injected into commit-message prompts."`
	PRInstructions     string `json:"prInstructions,omitempty" jsonschema:"Guidance injected into pull-request prompts."`
}

// OrchestratorSpec shapes the agent that is born with the workspace. The three
// dials become prose in the orchestrator's instruction document, which is why
// they are enumerated rather than free text: the prompt has to say something
// specific for each value.
type OrchestratorSpec struct {
	Name     string  `json:"name,omitempty" jsonschema:"Display name of the orchestrator. Defaults to \"Atlas\"."`
	Tone     string  `json:"tone,omitempty" jsonschema:"One of: efficient, friendly, professional, candid." validate:"omitempty,oneof=efficient friendly professional candid"`
	Style    string  `json:"style,omitempty" jsonschema:"One of: concise, balanced, detailed." validate:"omitempty,oneof=concise balanced detailed"`
	Autonomy float64 `json:"autonomy,omitempty" jsonschema:"How independently the orchestrator acts, from 0 to 1." validate:"omitempty,gte=0,lte=1"`

	// Sandbox overrides what the orchestrator may do. Omitted, it gets
	// DefaultOrchestratorSandbox — which is a working agent rather than a
	// read-only one, for the reason buildOrchestrator gives.
	Sandbox *SandboxSpec `json:"sandbox,omitempty" jsonschema:"What the orchestrator may do. Omit for the working default: read, write, delete, and execution from an allowlist."`
}

// SandboxSpec is the caller's own sandbox for the orchestrator.
type SandboxSpec struct {
	Permissions []string  `json:"permissions,omitempty" jsonschema:"Any of: read, write, delete, execute."`
	Exec        *ExecSpec `json:"exec,omitempty" jsonschema:"Which programs it may run."`
}

// ExecSpec is the execution half of a caller-supplied sandbox.
type ExecSpec struct {
	Policy     string   `json:"policy,omitempty" jsonschema:"allowlist or deny-all."`
	Allow      []string `json:"allow,omitempty" jsonschema:"Binary names this agent may run."`
	AllowShell bool     `json:"allowShell,omitempty" jsonschema:"Whether it may reach a shell. A shell makes the allowlist a suggestion."`
}

// The defaults of a new workspace, matching the original value for value. The
// one deliberate change is the branch prefix, which carries the product name.
var (
	// DefaultColor is the accent the original assigns when none is given.
	DefaultColor = "#5E6AD2"

	// DefaultTaskTypes is the five-type taxonomy every workspace starts with.
	DefaultTaskTypes = []TaskType{
		{ID: "feature", Label: "Feature", Color: "#6366f1"},
		{ID: "bug", Label: "Bug", Color: "#ef4444"},
		{ID: "refactor", Label: "Refactor", Color: "#f59e0b"},
		{ID: "docs", Label: "Docs", Color: "#10b981"},
		{ID: "config", Label: "Config", Color: "#64748b"},
	}
)

// DefaultGit returns the version-control policy of a fresh workspace.
func DefaultGit() GitOptions {
	return GitOptions{BranchPrefix: "aos", ForcePush: false}
}

// DefaultWorktrees returns the isolation policy of a fresh workspace.
func DefaultWorktrees() WorktreeOptions {
	return WorktreeOptions{DeleteOldWorktrees: true, WorktreeLimit: 15}
}

// defaultTaskTypes returns a copy, so that a workspace mutating its taxonomy
// cannot reach the package-level slice every other workspace starts from.
func defaultTaskTypes() []TaskType {
	out := make([]TaskType, len(DefaultTaskTypes))
	copy(out, DefaultTaskTypes)
	return out
}

// TaskTypeOf returns the taxonomy entry with the given id.
func (w Workspace) TaskTypeOf(id string) (TaskType, bool) {
	for _, t := range w.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return TaskType{}, false
}
