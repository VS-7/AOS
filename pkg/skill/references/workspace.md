# Workspace

Register and inspect the repositories this system operates on.

Manage workspaces.

A workspace is a repository this system operates on. Its metadata lives under
the installation state directory; its data lives inside the repository itself,
under `.aos/`, so that the state of an agent is committed
next to the code it works on.

Creating a workspace lays out that directory, puts the repository under version
control if it is not already, and creates the orchestrator agent that answers
when no other agent is addressed.

## When to use
- To register the project you are working in, with `introspect`
- To find out what a workspace already contains, with `inventory`
- To change the task taxonomy, the branch prefix or the Git policy

## When NOT to use
- Not to store knowledge — that is what memories are for
- Not to configure the installation — that is `config`, which is global

## Commands

### `workspace_create`

Register a workspace and lay it out.

Register a repository as a workspace.

The identifier is the slug of the name, so "Project Alpha" becomes
"project-alpha". Give a path to bind the workspace to a repository you already
have; omit it and one is created under the state directory.

Three things happen beyond the registry entry: the `.aos/` layout is
created inside the repository, the repository is put under version control if it
is not already, and the orchestrator agent is created. The result reports each
of them, because none of them is guaranteed to have been needed.

- register the repository you are in
- shape the orchestrator while creating it

### `workspace_delete`

Unregister a workspace.

Unregister a workspace.

This removes the registry entry and the derived state kept under the
installation directory. It does not touch your repository: the `.aos/`
directory in it holds agents, memories and instructions that you wrote, and
unregistering is not a request to delete them.

### `workspace_get`

Read one workspace.

Read the record of a workspace: its path, its branding, its task taxonomy
and its Git policy.

With no id, this reads the active workspace — the one named by the environment,
which is what the managed block in the repository's .env sets.

- the active workspace
- a specific one

### `workspace_introspect`

Register the repository you are standing in.

Register the current repository as a workspace, deriving its name from the
Git remote and falling back to the directory name.

Running this twice is safe and is the normal thing to do: a repository that is
already registered is returned unchanged.

- register the current directory

### `workspace_inventory`

See what a workspace holds, by collection.

Survey a workspace: how many agents, memories, tasks and instructions it
holds, and what they are called.

This is the panoramic view to read at the start of a session, before deciding
what to look at. It returns names and counts, never bodies — reading everything
to find out what exists is the most expensive way to answer the cheapest
question.

- survey the active workspace

### `workspace_list`

List the registered workspaces.

List every workspace registered on this installation.

Archived workspaces are left out unless asked for. Ordered by id, so the list
is stable between calls.

- the active workspaces
- including the archived ones

### `workspace_update`

Change fields of a workspace.

Change one or more fields of a workspace, addressed by dotted path.

Only the paths you send are changed. The identifier, the path and the creation
timestamp are the server's: a patch that reached them would orphan every record
that refers to this workspace.

- change the branch prefix and the accent colour

