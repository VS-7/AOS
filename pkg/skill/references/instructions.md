# Instructions

Durable, workspace-wide behavioral policy — rules every agent follows, not just the one that wrote them.

An instruction is workspace-wide policy: it shapes every agent, and its own
memories and preferences never override it. Use instructions for a
correction the user wants to apply to the whole workspace; use memory
(memories_store) for a correction about your own personal behavior. See
memories_store's own doc for the other half of that split.

## Commands
- **list** — every instruction, optionally filtered by skill or a text query
- **get** — one instruction's full content
- **create** — declare a new instruction
- **update** — change an existing instruction's fields
- **delete** — remove an instruction

## When to use
- **The user establishes a policy that should apply to every agent:** create
  an instruction, not a memory
- **Before assuming a behavior is already covered:** list first — an
  instruction with overlapping Paths may already say what you were about to
  create again

## When NOT to use
- Not for something true only of your own behavior — that is memories_store
- Not to bypass a workspace-wide rule you disagree with: an instruction is
  policy, and changing it is a shared-state action other agents will also be
  bound by

## Commands

### `instructions_create`

Declare a new workspace-wide instruction.

A shared-state change: the instruction created here shapes every agent in
the workspace, not just the one that called this. id is derived from name
when not given explicitly.

- a workspace-wide standard

### `instructions_delete`

Remove an instruction.

Remove an instruction's declaration. Idempotent: deleting what is already gone succeeds rather than erroring.

- retire a rule that no longer applies

### `instructions_get`

Read one instruction in full.

Read an instruction's complete content, the part instructions_list omits by name only.

- read before applying it

### `instructions_list`

List instructions, optionally filtered.

Every instruction in the workspace, or the subset matching a skill or a text query.

- everything active in the workspace
- one skill's own instructions

### `instructions_update`

Change an existing instruction.

A field left nil is unchanged; Paths, given at all, replaces the field wholesale.

- narrow an instruction to one directory

