# Toolsets

External connections — MCP servers, REST APIs, CLIs — reached through one call.

A toolset is configured once and called by id thereafter. Its own tools are
never registered into the agent's tool list — they are reached only through
toolsets_call, which is the one place this system executes something outside
its own process.

## Commands
- **list** — every configured toolset
- **get** — one toolset's full configuration
- **get-config** — the same read, plus which of its variables are still missing
- **call** — run one of its tools
- **update-config** — reconfigure it
- **delete** — remove it

## When to use
- **Before calling a tool you have not called yet:** get it first — the
  configuration and status tell you whether it is even reachable
- **A capability outside what this system's own domains cover:** call
  toolsets_call rather than assuming a native command exists

## When NOT to use
- Not to reach a native domain — toolsets are for integrations this system
  does not itself implement

## Commands

### `toolsets_call`

Run one tool of a connected toolset.

The one boundary where this system executes something outside its own
process: connects, calls one tool, closes, and audits the attempt regardless
of outcome.

Input and the result are opaque to this service — call toolsets_get first if
the tool's own argument shape is not already known.

- call a tool with no arguments

### `toolsets_delete`

Remove a toolset.

Remove a toolset's configuration. Idempotent: deleting what is already gone succeeds rather than erroring.

- remove a toolset

### `toolsets_get`

Read one toolset's configuration.

Read a toolset in full: its connection settings and its lifecycle status.

- read a toolset before calling it

### `toolsets_get-config`

Read one toolset's configuration, and what it still needs.

The same toolset toolsets_get answers with — a configuration is not split
into a separate document — plus the variables it needs before it can connect,
each marked set or missing.

The values are never returned: these are the credentials the toolset connects
with, and knowing which are still missing is what somebody configuring it
needs. Until this answered them, the only way to find out was to connect and
read the error, one variable at a time.

- read before reconfiguring

### `toolsets_list`

List every configured toolset.

Every toolset configured in this workspace, with its type and lifecycle status.

- everything configured

### `toolsets_update-config`

Reconfigure a toolset.

Change the describable parts of a toolset's configuration. A field left
nil is unchanged; Args, Env and Headers, given at all, replace the field
wholesale — there is no per-key merge.

- disable a misbehaving toolset

