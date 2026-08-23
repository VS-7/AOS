# Skills

Installable packages of capability.

A skill is not documentation: it brings agents, memories, routines,
collections, views and toolsets together as one unit, verified against its
own declared manifest and applied only once a person has agreed.

## Commands
- **list** — every skill installed in this workspace
- **install** — verify a package's manifest, ask a person, and apply it
- **create** — the same operation, named for a script or an agent assembling
  a package from a local directory rather than a person clicking install
- **update** — turn a skill's hooks and toolsets on or off, without removing it
- **delete** — uninstall a skill and everything it brought

## When to use
- **A capability that should persist across sessions, with its own agents and
  data:** install it as a skill rather than improvising the pieces by hand

## When NOT to use
- Not for a single collection or view with no agent behind it — declare that
  directly instead of packaging it

## Commands

### `skills_create`

Install a skill package, from a script or an agent assembling one.

The same operation as skills_install, under the name a script or an agent
building a package from a local directory reaches for. A person is still
asked before anything is written — this name changes who is calling, not
what is checked.

- install a package just assembled on disk

### `skills_delete`

Uninstall a skill.

Removes a skill and everything it brought: its agents, memories,
routines, collections, views, hooks and toolset connections.

Hooks and toolsets are torn down first, so nothing keeps intercepting a tool
call or holding a connection on behalf of a directory about to disappear.

- uninstall a skill

### `skills_install`

Install a skill package.

Installs a capability: the agents, collections, views, routines and
memories it ships, as one unit.

The package's manifest is verified against what it actually contains before
anything is written, and a person is asked before it is applied. An agent
does not authorise this on its own.

- install from a local directory
- install a specific version

### `skills_list`

List every installed skill.

Every skill installed in this workspace, active or not, with what it brought.

- everything installed

### `skills_update`

Turn a skill's live behaviour on or off.

Enable or disable a skill without removing it: its agents, memories and
configuration stay in place, only its hooks and toolsets stop being live.

Nothing else about an installed skill changes here — content, permissions
and inventory are what an install verified and a person consented to; to
change any of those, uninstall and install again.

- disable a skill
- re-enable it

