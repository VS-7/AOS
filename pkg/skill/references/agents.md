# Agents

Create, read and change the agents of the workspace.

Manage the agents of this workspace.

An agent is a persistent identity: a slug, a role, a model, and Markdown
instructions that shape how it works. It is stored as `.aos/agents/{id}/AGENT.md`,
which means an agent is versionable in Git and editable by hand.

## When to use
- To see who is available before delegating work
- To read an agent's instructions before changing them
- To create a specialist for a recurring kind of task

## When NOT to use
- Not to store knowledge — that is what memories are for
- Not to track work — that is what tasks are for

## Commands

### `agents_create`

Create an agent.

Create an agent in this workspace.

The slug is the identity: it names the directory, and it is what every other
record refers to. It is lowercased on write, so "Atlas" and "atlas" are the same
agent rather than two.

- a reviewer bound to a specific model

### `agents_delete`

Delete an agent and everything under it.

Delete an agent.

This removes the agent's whole directory: its memories, its routines and its
event log go with it. There is no undo other than Git.

### `agents_get`

Read one agent, instructions included.

Read one agent by slug.

Returns the whole record: identity, model binding, channels and the Markdown
body that is the agent's system instruction.

- read the orchestrator

### `agents_list`

List the agents of the workspace.

List every agent in the workspace, including the ones a skill shipped.

Returns the identity of each agent, not its instructions: use `agents get` for
the full record. Ordered by slug, so the list is stable between calls.

- every agent
- only the reviewers

### `agents_me`

Resolve your own identity in this workspace.

Return the full record of whoever is calling.

Inside an agent execution that is the agent itself; from a human terminal it is
the workspace orchestrator. There is no argument: the identity comes from the
execution context, which is the point — an agent that could name itself could
name another.

Use it at the start of a session to confirm which instructions are actually
loaded. Most surprising behaviour turns out to be a different agent than the
one you thought was running.

- who am I right now

### `agents_update`

Change an agent.

Change one or more fields of an agent.

Only the fields you send are changed; the rest are left alone. Sending
`content` replaces the instructions entirely — read them first.

