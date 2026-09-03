package workspace

import (
	"context"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "workspace",
	Tool:    "Workspace",
	Summary: "Register and inspect the repositories this system operates on.",
	Doc: `Manage workspaces.

A workspace is a repository this system operates on. Its metadata lives under
the installation state directory; its data lives inside the repository itself,
under ` + "`" + collections.Root + "/`" + `, so that the state of an agent is committed
next to the code it works on.

Creating a workspace lays out that directory, puts the repository under version
control if it is not already, and creates the orchestrator agent that answers
when no other agent is addressed.

## When to use
- To register the project you are working in, with ` + "`introspect`" + `
- To find out what a workspace already contains, with ` + "`inventory`" + `
- To change the task taxonomy, the branch prefix or the Git policy

## When NOT to use
- Not to store knowledge — that is what memories are for
- Not to configure the installation — that is ` + "`config`" + `, which is global`,
	Hint: "One workspace has at most one orchestrator. Registering over an existing layout adopts what is there rather than creating a second one.",
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "workspace",
		Name:    "list",
		Aliases: []string{"ls"},
		Summary: "List the registered workspaces.",
		Doc: `List every workspace registered on this installation.

Archived workspaces are left out unless asked for. Ordered by id, so the list
is stable between calls.`,
		Examples: []command.Example{
			{Description: "the active workspaces", Input: ListInput{}},
			{Description: "including the archived ones", Input: ListInput{IncludeArchived: true}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List workspaces", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Workspace]{
		Group:   "workspace",
		Name:    "get",
		Summary: "Read one workspace.",
		Doc: `Read the record of a workspace: its path, its branding, its task taxonomy
and its Git policy.

With no id, this reads the active workspace — the one named by the environment,
which is what the managed block in the repository's .env sets.

The identifier is accepted under either name: ` + "`workspace`" + `, which this group has
always published, or ` + "`id`" + `, which is what every other group calls the identifier
of its own resource.`,
		Examples: []command.Example{
			{Description: "the active workspace", Input: GetInput{}},
			{Description: "a specific one", Input: GetInput{Workspace: "project-alpha"}},
			{Description: "the same, under the name the other groups use", Input: GetInput{ID: "project-alpha"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a workspace", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, CreateOutput]{
		Group:   "workspace",
		Name:    "create",
		Summary: "Register a workspace and lay it out.",
		Doc: `Register a repository as a workspace.

The identifier is the slug of the name, so "Project Alpha" becomes
"project-alpha". Give a path to bind the workspace to a repository you already
have; omit it and one is created under the state directory.

Three things happen beyond the registry entry: the ` + "`" + collections.Root + "/`" + ` layout is
created inside the repository, the repository is put under version control if it
is not already, and the orchestrator agent is created. The result reports each
of them, because none of them is guaranteed to have been needed.`,
		Examples: []command.Example{
			{
				Description: "register the repository you are in",
				Input:       CreateInput{Name: "Project Alpha", Path: "/home/me/project-alpha"},
			},
			{
				Description: "shape the orchestrator while creating it",
				Input: CreateInput{
					Name: "Project Alpha", Path: "/home/me/project-alpha",
					Orchestrator: &OrchestratorSpec{
						Name: "Atlas", Tone: "candid", Style: "concise", Autonomy: 0.8,
					},
				},
			},
		},
		Registry:    false,
		Annotations: command.Annotations{Title: "Create a workspace"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Workspace]{
		Group:   "workspace",
		Name:    "update",
		Summary: "Change fields of a workspace.",
		Doc: `Change one or more fields of a workspace, addressed by dotted path.

Only the paths you send are changed. The identifier, the path and the creation
timestamp are the server's: a patch that reached them would orphan every record
that refers to this workspace.

Name the workspace with ` + "`workspace`" + ` or ` + "`id`" + `. With neither, this changes the
active one — the workspace this session is scoped to.`,
		Examples: []command.Example{
			{
				Description: "change the branch prefix and the accent colour",
				Input: UpdateInput{Set: map[string]any{
					"git.branchPrefix": "feat",
					"color":            "#10b981",
				}},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a workspace", IdempotentHint: true},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "workspace",
		Name:    "delete",
		Summary: "Unregister a workspace.",
		Doc: `Unregister a workspace.

This removes the registry entry and the derived state kept under the
installation directory. It does not touch your repository: the ` + "`" + collections.Root + "/`" + `
directory in it holds agents, memories and instructions that you wrote, and
unregistering is not a request to delete them.`,
		// Not in the agent's registry, with create below it. Registering and
		// unregistering a workspace is the shape of the installation, not
		// work inside one — the same boundary command.Command's Registry
		// field draws around gateway, auth and tunnels. Both remain on the
		// CLI, over MCP and over HTTP, which is where a person (or a coding
		// agent operating AOS from outside) does this.
		Registry:    false,
		Annotations: command.Annotations{Title: "Unregister a workspace", DestructiveHint: true},
		Handler:     svc.Delete,
	})

	command.MustRegister(reg, command.Command[IntrospectInput, CreateOutput]{
		Group:   "workspace",
		Name:    "introspect",
		Summary: "Register the repository you are standing in.",
		Doc: `Register the current repository as a workspace, deriving its name from the
Git remote and falling back to the directory name.

Running this twice is safe and is the normal thing to do: a repository that is
already registered is returned unchanged, and this command never answers
"already exists" — a directory whose repository name is taken by another
registration is registered under a distinct identifier instead.

"The current repository" is the directory the *caller* is standing in, which
the terminal reports with every call. A client that has no directory of its own
— the window, a browser — gets the one the daemon was started against, and can
always name a path explicitly.`,
		Examples: []command.Example{
			{Description: "register the current directory", Input: IntrospectInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Register the current repository", IdempotentHint: true},
		Handler:     svc.Introspect,
	})

	command.MustRegister(reg, command.Command[InventoryInput, Inventory]{
		Group:   "workspace",
		Name:    "inventory",
		Summary: "See what a workspace holds, by collection.",
		Doc: `Survey a workspace: how many agents, memories, tasks and instructions it
holds, and what they are called.

This is the panoramic view to read at the start of a session, before deciding
what to look at. It returns names and counts, never bodies — reading everything
to find out what exists is the most expensive way to answer the cheapest
question.`,
		Examples: []command.Example{
			{Description: "survey the active workspace", Input: InventoryInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Survey a workspace", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Inventory,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)         = (*Service)(nil).List
	_ func(context.Context, CreateInput) (CreateOutput, error)     = (*Service)(nil).Create
	_ func(context.Context, InventoryInput) (Inventory, error)     = (*Service)(nil).Inventory
	_ func(context.Context, IntrospectInput) (CreateOutput, error) = (*Service)(nil).Introspect
)
