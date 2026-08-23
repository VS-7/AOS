---
name: aos
description: Use when you need persistent memory across sessions, lifecycle-managed
  tasks, specialized agents, structured data collections, views, scheduled
  routines, or installable skills. Triggers: "remember this", "what did we
  decide about X", "create a task", "delegate to", "schedule", "build a CRM".
---

# AOS

Infrastructure layer that gives agents persistent memory, lifecycle
execution and continuity across sessions.

## Session start — mandatory

Before any substantive work:

1. `agents_me` — find out who you are in this workspace
2. `memories_recall` (limit 20) — retrieve what you already know
3. `workspace_introspect` — see what exists

## Memory protocol

**Recall before you store.** If a trace already exists, link or supersede it
— duplicates dilute the graph and confuse future recall.

**Calibrate confidence honestly:** 0.9-1.0 verified · 0.6-0.8 strong inference
· below 0.6 guess. Inflated confidence is the main way you mislead your
future self.

**Memories are global across your parallel instances.** There is no draft:
what you write, every parallel self sees, and a deprecation affects all of
them immediately.

**Before delivering a final answer, before moving a task to in_review, and
before completing a routine:** reflect on what you learned and keep your
memories current.

## Composite tools — inspect before you call

Many tools group several actions under one `action` field. The tool's own
description is only the group's overview; each action has its own
description, examples and input schema.

On the **first** call to each action in a session, pass `schema: true` at the
same level as `action` to get the full specification. After that you know
the contract and can call directly.

If a call fails validation, do not retry blindly — read the error, inspect
the contract with `schema: true`, fix the payload.

## Routing

| I need... | Read |
|---|---|
| Memory, learning, a decision | `references/memories.md` |
| Lifecycle-managed work | `references/tasks.md` |
| Delegate to a specialist | `references/agents.md` |
| Structured data | `references/collections.md` |
| An interface over data | `references/views.md` |
| Scheduled automation | `references/routines.md` |
| An external tool | `references/toolsets.md` |
| A publishable deliverable | `references/artifacts.md` |
| Workspace policy | `references/instructions.md` |

## Hard rules

- Every tool call requires a non-empty `_reasoning`
- Use `set-status` to move a task; never `update`
- In task mode, communicate through the task's own comments, not chat
- Only move to in_review with validation evidence — the system refuses the
  transition with everything still pending
- An instruction is workspace policy; a memory is yours. Personal correction
  → memory. Broad-scope correction → instruction, and it goes through human
  approval
- A command outside your agent's allowlist fails; the error says exactly
  what to ask the workspace owner for
- An action that asks for approval genuinely asks a human when a channel is
  available. In headless mode the denial is immediate and explicit
