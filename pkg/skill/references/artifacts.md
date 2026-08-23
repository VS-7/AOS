# Artifacts

Static web applications published from the workspace — dashboards, reports, landing pages — served with no deploy step.

An artifact is a small static site: an entrypoint HTML file and whatever
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
  reachable by whoever the visibility allows, unlike a chat message

## Commands

### `artifacts_create`

Register a new artifact.

Registers a new artifact and scaffolds a minimal entrypoint HTML file when
none is given. Defaults to private visibility.

- publish a new dashboard

### `artifacts_delete`

Remove an artifact.

Remove an artifact's registration and its files. Idempotent: deleting what is already gone succeeds rather than erroring.

- remove an artifact

### `artifacts_get`

Read one artifact's configuration.

Read an artifact in full: its entrypoint, visibility and metadata.

- read an artifact before editing it

### `artifacts_list`

List every artifact registered in the workspace.

Every artifact registered in this workspace, with its visibility and entrypoint.

- everything published

### `artifacts_set-password`

Set the password a by_password artifact is shared behind.

Hashes the given password with argon2id and persists the hash, so the
returned URL keeps working after a restart. Does not change visibility —
call update first if the artifact is not already by_password.

- share an artifact by link

### `artifacts_update`

Change an artifact's configuration.

Change the describable parts of an artifact. A field left nil is unchanged.

- make an artifact visible workspace-wide

