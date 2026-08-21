package artifact

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "artifacts",
	Tool:    "Artifacts",
	Summary: "Static web applications published from the workspace — dashboards, reports, landing pages — served with no deploy step.",
	Doc: `An artifact is a small static site: an entrypoint HTML file and whatever
else it references, registered in the workspace and served by this daemon at
/v/{workspace}/artifacts/{id}/*.

## Commands
- **list** — every artifact registered in the workspace
- **get** — one artifact's configuration
- **create** — register a new one, scaffolding a minimal entrypoint if none is given
- **update** — change its name, description, entrypoint or visibility
- **set-password** — set the password a by_password artifact is shared behind
- **delete** — remove it and its files

## When to use
- **Publishing something for a person to open in a browser:** a dashboard,
  report or generated page an agent produced — create an artifact rather than
  describing the content in a message
- **Sharing outside the workspace:** set visibility to by_password and call
  set-password, then hand out the URL set-password returns

## When NOT to use
- Not for anything that should stay inside the conversation — an artifact is
  reachable by whoever the visibility allows, unlike a chat message`,
	Hint: `Content itself — the HTML, CSS and JS the artifact serves — is written to
its directory through the file tools, not through this group; these commands
manage the artifact's registration and access, not its files.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, []Artifact]{
		Group:   "artifacts",
		Name:    "list",
		Summary: "List every artifact registered in the workspace.",
		Doc:     "Every artifact registered in this workspace, with its visibility and entrypoint.",
		Examples: []command.Example{
			{Description: "everything published", Input: ListInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List artifacts", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Artifact]{
		Group:   "artifacts",
		Name:    "get",
		Summary: "Read one artifact's configuration.",
		Doc:     "Read an artifact in full: its entrypoint, visibility and metadata.",
		Examples: []command.Example{
			{Description: "read an artifact before editing it", Input: GetInput{ID: "sales-dashboard"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read an artifact", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Artifact]{
		Group:   "artifacts",
		Name:    "create",
		Summary: "Register a new artifact.",
		Doc: `Registers a new artifact and scaffolds a minimal entrypoint HTML file when
none is given. Defaults to private visibility.`,
		Examples: []command.Example{
			{Description: "publish a new dashboard", Input: CreateInput{Name: "Sales dashboard"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create an artifact"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Artifact]{
		Group:   "artifacts",
		Name:    "update",
		Summary: "Change an artifact's configuration.",
		Doc:     "Change the describable parts of an artifact. A field left nil is unchanged.",
		Examples: []command.Example{
			{Description: "make an artifact visible workspace-wide", Input: UpdateInput{ID: "sales-dashboard", Visibility: visibilityPtr(Workspace)}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update an artifact"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[SetPasswordInput, SetPasswordOutput]{
		Group:   "artifacts",
		Name:    "set-password",
		Summary: "Set the password a by_password artifact is shared behind.",
		Doc: `Hashes the given password with argon2id and persists the hash, so the
returned URL keeps working after a restart. Does not change visibility —
call update first if the artifact is not already by_password.`,
		Examples: []command.Example{
			{Description: "share an artifact by link", Input: SetPasswordInput{ID: "sales-dashboard", Password: "correct-horse-battery-staple"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Set an artifact's password", DestructiveHint: true},
		Handler:     svc.SetPassword,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "artifacts",
		Name:    "delete",
		Summary: "Remove an artifact.",
		Doc:     "Remove an artifact's registration and its files. Idempotent: deleting what is already gone succeeds rather than erroring.",
		Examples: []command.Example{
			{Description: "remove an artifact", Input: DeleteInput{ID: "sales-dashboard"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete an artifact", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func visibilityPtr(v Visibility) *Visibility { return &v }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) ([]Artifact, error)               = (*Service)(nil).List
	_ func(context.Context, GetInput) (*Artifact, error)                 = (*Service)(nil).Get
	_ func(context.Context, CreateInput) (*Artifact, error)              = (*Service)(nil).Create
	_ func(context.Context, UpdateInput) (*Artifact, error)              = (*Service)(nil).Update
	_ func(context.Context, SetPasswordInput) (SetPasswordOutput, error) = (*Service)(nil).SetPassword
	_ func(context.Context, DeleteInput) (DeleteOutput, error)           = (*Service)(nil).Delete
)
