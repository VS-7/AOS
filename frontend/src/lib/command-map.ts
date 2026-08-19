import type { CommandKey } from "./schema";
import * as fileApi from "./file";
import * as authApi from "./auth";

/**
 * One Fractal frontend call, resolved.
 *
 * - `CommandKey` — goes through the command registry (`client.invoke`).
 * - `CommandDescriptor` — same, plus the adaptations a Go command
 *   sometimes needs (see below).
 * - `HttpHandler` — goes through its own HTTP surface (`/api/auth`,
 *   `/api/file`), which sits outside the registry by backend decision.
 * - `null` — the Go side does not have this yet. See the dormancy contract.
 */
export type HttpHandler = (payload: Record<string, unknown>) => Promise<unknown>;

/**
 * A command whose Go contract diverges from what the ported frontend sends
 * or expects back, described declaratively instead of patched at each call
 * site.
 *
 * This exists because a plain string couldn't say what command-map.ts
 * needed to say, discovered only once real ported pages exercised real Go
 * commands: many Fractal inputs use a field name Go's command doesn't
 * (`task` where Go wants `id`), many send a value of the wrong *type*
 * (an array where Go wants one scalar, a quoted number, an object where
 * Go wants a bare `bool`), and many Go commands answer with a bare entity
 * where the ported code expects it nested under a domain key (`tasks_get`
 * returns the task itself; `client.task.getById.query(...)` reads
 * `result.data.task`). Fixing any of these by hand-editing every call
 * site would mean 25 more features silently reinventing the same fix, or
 * forgetting to.
 */
export interface CommandDescriptor {
  /** The registry key `client.invoke` actually calls. */
  key: CommandKey;
  /**
   * Payload keys to rename before the call reaches Go — `{ fractalName:
   * goName }`. Applied to the flattened `{...params, ...query, ...body}`
   * object, so it doesn't matter which of the three a field started in.
   * Runs before `coerceIn`. `useQuery`'s cache key is computed from the
   * *original* payload (what the calling code actually passed, before
   * either transform) — both are purely outbound transport concerns,
   * invisible to the ported code on both sides.
   */
  renameIn?: Record<string, string>;
  /**
   * Type/shape fixes to apply to individual payload fields before the
   * call reaches Go, keyed by the field name the ported UI actually sends
   * (pre-`renameIn`, so name it the same as the UI's own key, not Go's).
   * Each function receives that field's current value:
   *
   * - Return a plain value to replace the field in place (a scalar Go
   *   wants where the UI holds an array — take the first element; an
   *   `int` where the UI sent a quoted string; etc).
   * - Return a plain *object* to replace the field entirely with that
   *   object's own keys, merged into the payload — for a UI field whose
   *   wire shape Go splits into several top-level ones (a `{enabled,
   *   base, branch}` object where Go wants a bare `worktree: bool` plus a
   *   separate top-level `base`, dropping `branch` because Go's `create`
   *   input has no field for it at all).
   *
   * This is for the wire contract only — a scalar/array/bool/int
   * mismatch between what the UI collects and what Go's JSON decoder
   * accepts. A transform that reshapes what the UI *means* (which of
   * several editable fields feeds which Go field, restructuring one
   * nested concept's rendering into different JSX) is a UI data-model
   * decision and stays a disclosed call-site edit, not a `coerceIn` entry
   * — see the `task.create`/`task.list` entries below for the line drawn
   * in practice.
   */
  coerceIn?: Record<string, (value: unknown) => unknown>;
  /**
   * Go returns a bare entity for this command (no wrapper object) — wrap
   * the response under this key so `result.data.<wrapOut>` resolves the
   * way the ported code expects. Applied only to a defined result; a
   * dormant or errored call is untouched.
   */
  wrapOut?: string;
}

export type MapEntry = CommandKey | CommandDescriptor | HttpHandler | null;

const s = (p: Record<string, unknown>, k: string): string => String(p[k] ?? "");

/**
 * The map is explicit, not a pluralization rule.
 *
 * A rule would fail silently on the irregular cases — and they are real:
 * `getById`→`get`, `markAsRead`→`read`, and two kebab-case ones
 * (`set-status`, `read-all`) that no pluralization would produce.
 *
 * `null` is a declaration, not an omission: it means "the Go side does not
 * have this command yet." A missing key is a programming error, and the
 * facade fails loudly in that case.
 */
export const COMMAND_MAP: Record<string, MapEntry> = {
  // ── command registry ────────────────────────────────────────────────────
  "activity.list": "activity_list",
  "activity.markAllAsRead": "activity_read-all",
  // task-12 live HTTP pass: `activity.store.ts`'s `markAsRead` action calls
  // `api.activity.markAsRead.mutate({ params: { activity: activityId } })` —
  // Go's `MarkInput` (`internal/domain/activity/schema.go`) names that
  // field `id`. Without `renameIn` the unknown `activity` key was dropped
  // and `id` arrived empty; `MarkInput.ID` has no `required` validator, so
  // this didn't 400 — it silently marked *no* entry (Go's `Get`-by-empty-id
  // path just doesn't find one) on every "mark as read" click.
  "activity.markAsRead": { key: "activity_read", renameIn: { activity: "id" } },
  // `agent.create`/`agent.update` answer with a bare `*Agent`
  // (`internal/domain/agent/commands.go`), not `{agent: ...}` — confirmed
  // against the Fractal source's own consumer (`agents.context.tsx` reads
  // `response.data?.agent`), not guessed; nothing in this app calls these
  // through the facade yet (no `agent` feature ported), so this was a
  // latent instance of the same bug `todo.getById` was, caught by the
  // full-map sweep the round-2 review asked for rather than by a live
  // failure.
  "agent.create": { key: "agents_create", wrapOut: "agent" },
  "agent.delete": "agents_delete",
  "agent.update": { key: "agents_update", wrapOut: "agent" },
  // `chat.getById`/`chat.create` answer with a bare `*Chat`
  // (`internal/domain/chat/commands.go`). Both are live and confirmed
  // correct as written — task-12's live HTTP pass exercised both directly:
  // `use-chat.ts` calls `aos.client.chat.getById.useQuery({ params: {
  // chat: chatId } })` and reads `data?.chat`, and
  // `create-channel-dialog.tsx` calls `chat.create` and reads
  // `response?.data?.chat`. (An earlier round of this comment claimed
  // neither call site went through this facade at all — that was wrong;
  // it was never re-checked against the actual call sites once `chat` was
  // ported, which is exactly the gap the live-exercise pass exists to
  // catch.)
  "chat.create": { key: "chats_create", wrapOut: "chat" },
  "chat.getById": { key: "chats_get", wrapOut: "chat" },
  "chat.list": "chats_list",
  // task-12 (round 2): `use-chat-composer.ts`'s `sendMessage` calls this
  // with `body: { id: chat.id, message: <FractalChatMessage: {id, role,
  // parts, metadata}> }` — a rich AI-SDK `UIMessage`, never a plain string.
  // Go's `SendInput` (`internal/domain/chat/schema.go`) has exactly three
  // fields — `chat`, `text` (required), `agent` — and no representation
  // for a message id, structured parts, or metadata at all.
  //
  // `ComposerHelper.buildMessageParts` (`presentation/helpers/
  // composer.helper.ts`) only ever constructs two part types for an
  // outbound user message: `text` (the typed text, plus any auto-attached
  // instruction references — themselves plain text, mentions and skill/
  // file references included, since those are serialized as inline markup
  // *inside* the text string by `ChatInlineMarkupHelper`, not a separate
  // part type) and `file` (real attachments, collected separately from
  // typed text). `coerceIn` joins every `text` part in order, which is a
  // faithful flatten — nothing meant to be read as text is lost. A `file`
  // part is dropped here **explicitly**: Go's `SendInput` has no field an
  // attachment could go in, so an attachment the composer collected does
  // not reach the server today. This is disclosed, not silent — surfacing
  // it further (e.g. rejecting the send, or showing the user their
  // attachment won't arrive) is a UI decision for whoever owns the
  // composer, not something this map can express.
  "chat.send": {
    key: "chats_send",
    coerceIn: {
      message: (value) => {
        const parts = (value as { parts?: Array<{ type: string; text?: string }> } | undefined)?.parts ?? [];
        const text = parts
          .filter((part) => part.type === "text" && typeof part.text === "string")
          .map((part) => part.text)
          .join("\n\n");
        return { text };
      },
    },
  },

  // `comment.list` was the only entry here before the `task` port. Go
  // registers a full `comments` command group — list, get, create, update,
  // delete (`internal/domain/comment/commands.go`) — the other four are
  // real, live commands, just never registered.
  //
  // Every one of Go's comment.* input structs names the parent task field
  // `task` (`internal/domain/comment/schema.go`); the ported code
  // (`comments/index.tsx`, `todo-widget/index.tsx`'s sibling pattern)
  // consistently calls it `taskId`. `comment.create` additionally sends
  // the reply-target field as `replyToId`, which exists on neither Go's
  // `CreateInput` (`parent`) nor the stored `Comment` (`parentId`).
  //
  // Go's `Comment` read model has no `body` field at all — the persisted
  // text is `content` (`internal/domain/comment/entity.go`). `body` is
  // only a *write* field name (`CreateInput.Body`, `UpdateInput.Body`),
  // which is why `renameIn` doesn't touch it: the outbound `body` key
  // already matches. Reading a fetched comment's text as `.content`
  // instead of `.body` is a call-site fix (see the port's diff notes),
  // not something this map can express.
  "comment.create": { key: "comments_create", renameIn: { taskId: "task", replyToId: "parent" }, wrapOut: "comment" },
  "comment.delete": { key: "comments_delete", renameIn: { taskId: "task" } },
  "comment.getById": { key: "comments_get", renameIn: { taskId: "task" }, wrapOut: "comment" },
  "comment.list": { key: "comments_list", renameIn: { taskId: "task" } },
  "comment.update": { key: "comments_update", renameIn: { taskId: "task" }, wrapOut: "comment" },

  "config.get": "config_get",
  "config.update": "config_update",
  // task-12 correction: the comments this whole `routine.*` block carried
  // said "no live consumer here yet (`routine` isn't a ported feature)".
  // That was wrong — `presentation/pages/($id)/index.tsx` is live and
  // calls every one of `getById`/`create`/`update`/`delete`/`fire` through
  // `aos.client.routine.*` — the earlier round's own verification never
  // actually exercised it. Found by task 12's live HTTP pass, not by
  // static reading: every one of `getById`/`update`/`delete`/`fire` sends
  // `params: { routine: id }`, but every one of Go's
  // `GetInput`/`UpdateInput`/`DeleteInput`/`FireInput`
  // (`internal/domain/routine/schema.go`) names that field `id` — with no
  // `renameIn`, the unknown `routine` key was silently dropped by Go's
  // decoder and the required `id` arrived empty, so every one of those
  // four 400'd on every call (`validate:"required,notblank"`). `create`
  // and `update`'s body also sends `prompt`; Go's field is `content`.
  "routine.create": { key: "routines_create", renameIn: { prompt: "content" } },
  "routine.delete": { key: "routines_delete", renameIn: { routine: "id" } },
  // NOT `wrapOut: "run"`, on purpose. Go's `routines_fire` returns a
  // single bare `*Run` (`internal/domain/routine/commands.go`), but the
  // live consumer (`onSuccess` in `pages/($id)/index.tsx`, matching
  // Fractal's original `routine-list-row.component.tsx`) reads
  // `result.data?.executions?.length` — a *list* of executions, not one
  // run under any single key. `wrapOut` can only rename/nest a value, not
  // turn one entity into an array, so this can't be fixed here — that
  // read is adapted at the call site instead (that file's own comment,
  // task-12 round 2): the single `Run` is wrapped in a one-element array
  // where it's read, so the count reflects what Go actually did rather
  // than silently defaulting to "1" regardless of outcome.
  "routine.fire": { key: "routines_fire", renameIn: { routine: "id" } },
  "routine.getById": { key: "routines_get", renameIn: { routine: "id" }, wrapOut: "routine" },
  "routine.list": "routines_list",
  "routine.update": { key: "routines_update", renameIn: { routine: "id", prompt: "content" } },

  // Every `tasks_*` command that names one task takes `id`
  // (`internal/domain/task/schema.go`'s `GetInput`/`UpdateInput`/
  // `SetStatusInput`/`DeleteInput`, all `ID string json:"id"`) — the
  // ported code uniformly calls it `task`. `tasks_get`/`tasks_create`/
  // `tasks_update` also answer with the bare task (`*View`), not `{task:
  // ...}`; `tasks_set-status` already answers `{task, from, to}` and
  // needs no `wrapOut`. `tasks_delete`'s `DeleteOutput{id, ...}` is read
  // by no ported call site, so it stays unwrapped too.
  //
  // `coerceIn` on `task.create`: `dialogs/create/index.tsx`'s form groups
  // worktree options as one editable `{enabled, base, branch}` object —
  // that grouping is the form's own UI decision and is untouched here.
  // What changes is purely the *wire* shape: `CreateInput.Worktree` is a
  // bare `bool`, `Base` is a separate top-level field, and there is no
  // `branch` field on create at all (branching is the separate
  // `task.branch` command) — so `branch` has nowhere to go and is
  // dropped, same as any other field Go's decoder doesn't recognize.
  "task.create": {
    key: "tasks_create",
    coerceIn: {
      worktree: (value) => {
        const w = value as { enabled?: boolean; base?: string } | undefined;
        return w?.enabled ? { worktree: true, ...(w.base ? { base: w.base } : {}) } : { worktree: false };
      },
    },
    wrapOut: "task",
  },
  "task.delete": { key: "tasks_delete", renameIn: { task: "id" } },
  "task.getById": { key: "tasks_get", renameIn: { task: "id" }, wrapOut: "task" },
  // `coerceIn` on `task.list`: `(main)/index.tsx`'s filter bar is a
  // genuine multi-select — that UI decision is untouched. What Go's
  // `ListInput` actually accepts for `type`/`project`/`goal` is one
  // scalar `string` each (`internal/domain/task/schema.go`), not a list —
  // an array into a string field is a hard 400 — so only the first
  // selection reaches Go; the rest of the UI's selection state is
  // unaffected, it just doesn't all filter server-side yet. `limit` is
  // `int`; the quoted-string form some call sites send fails Go's strict
  // unmarshal the same way.
  "task.list": {
    key: "tasks_list",
    coerceIn: {
      type: (value) => (Array.isArray(value) ? value[0] : value),
      project: (value) => (Array.isArray(value) ? value[0] : value),
      goal: (value) => (Array.isArray(value) ? value[0] : value),
      limit: (value) => (typeof value === "string" && value.trim() !== "" ? Number(value) : value),
    },
  },
  "task.setStatus": { key: "tasks_set-status", renameIn: { task: "id" } },
  "task.update": { key: "tasks_update", renameIn: { task: "id" }, wrapOut: "task" },

  // task-12 live HTTP pass: `theme.store.ts`'s `setPreset`/`update` actions
  // call `api.theme.get.query({ params: { theme: preset } })`. Go's
  // `GetInput` (`internal/domain/theme/schema.go`) names that field `id`,
  // with `validate:"required,notblank"` — every call 400'd. `renameIn`
  // fixes that wire mismatch.
  //
  // The response shape (`response.data?.theme?.theme` reading past Go's
  // real `theme.variants.{light,dark}`) can't be fixed here — `wrapOut`
  // only adds one nesting level to a bare entity, it can't rename a key
  // several levels inside an already-shaped `Output` struct — so that part
  // is a call-site fix (`theme.store.ts`'s own `paletteFromApi` and its
  // two callers), ruled on explicitly (task-12 round 2's side-by-side
  // comparison) rather than guessed: five of six `Palette` fields already
  // match `FractalThemeSettings` by name, `semantic`/`semanticColors` is a
  // plain rename, and `fonts` is a genuine gap — Go's `Palette` has no such
  // field at all, made visible with a comment at the read site rather than
  // silently defaulted.
  "theme.get": { key: "themes_get", renameIn: { theme: "id" } },
  "theme.list": "themes_list",

  // Only `todo.list` existed before the `task` port; `create`/`update`
  // were entirely unregistered even though `todos_create`/`todos_update`
  // are real, live commands (`internal/domain/todo/commands.go`).
  // `getById`/`setStatus`/`delete` are registered here too though nothing
  // ported calls them yet, for the same completeness reason `comment.*`
  // got its missing four.
  //
  // Every `todos_*` input names the parent task `task`
  // (`internal/domain/todo/schema.go`); the ported code calls it `taskId`
  // (`todo-widget/index.tsx`, `todo-dialog-upsert.tsx`). Go's `Todo` has
  // no `description`/`agent`/`instructions` fields — it has `title` and
  // `content` (`internal/domain/todo/entity.go`); `renameIn` covers the
  // two that map cleanly (`description`→`title`, `instructions`→
  // `content`). There is no Go equivalent for `agent` at all — a todo
  // cannot be assigned to a specific agent server-side yet, so that field
  // is sent and silently ignored (Go's JSON decoder drops unknown keys);
  // the widget still displays it if a locally-held value has one, but
  // nothing persists it. `todos_create`/`todos_update` answer with the
  // bare `*Todo`, hence `wrapOut`.
  "todo.create": { key: "todos_create", renameIn: { taskId: "task", description: "title", instructions: "content" }, wrapOut: "todo" },
  "todo.delete": { key: "todos_delete", renameIn: { taskId: "task" } },
  // Missing `wrapOut` here was the round-2 review's Important #1:
  // `todos_get` is `command.Command[GetInput, *Todo]` (bare), the exact
  // same shape as `comment.getById` right above, which already has
  // `wrapOut: "comment"`.
  "todo.getById": { key: "todos_get", renameIn: { taskId: "task" }, wrapOut: "todo" },
  "todo.list": { key: "todos_list", renameIn: { taskId: "task" } },
  "todo.setStatus": { key: "todos_set-status", renameIn: { taskId: "task" } },
  "todo.update": { key: "todos_update", renameIn: { taskId: "task", description: "title", instructions: "content" }, wrapOut: "todo" },

  "workspace.create": "workspace_create",
  "workspace.delete": "workspace_delete",
  "workspace.list": "workspace_list",
  // `workspace.update` was entirely absent from this map — found by the
  // task-12 call-path sweep, not by anyone clicking. Four settings forms
  // (`workspace/{git,profile,tasks,worktrees}/index.tsx`, under
  // `presentation/components/settings/components/sections/workspace`) call
  // it via `aos.useForm({ mutation: "workspace.update" })`
  // (`app/builders/app.tsx` splits that string and indexes into the same
  // `client[controller][action]` this map resolves); every one of the four
  // threw `call not mapped` on every submit.
  //
  // Go's `UpdateInput` (`internal/domain/workspace/schema.go`) takes
  // `workspace` (id) plus one dotted-path `set: map[string]any` — the same
  // pattern `config.update` already uses. `renameIn` covers `params: { id
  // }` -> `workspace`, the field name Fractal's original UI still uses.
  // Three of the four forms already submit one grouped object per top-level
  // key that lines up with a nested `Workspace` field (`git` ->
  // `GitOptions`, `worktrees` -> `WorktreeOptions`, `tasks` -> the `Tasks
  // []TaskType` field verbatim) — `coerceIn` turns each into the
  // corresponding `set` entries, a wire-shape fix (object -> dotted map),
  // not a UI data-model change.
  //
  // `.../profile` is not covered by `coerceIn` here: it submits three
  // independent top-level scalars (`name`, `logo`, `color`) in one call,
  // and `coerceIn`'s per-field merge (`aos-facade.ts`'s `applyCoerceIn`)
  // shallow-`Object.assign`s each field's result — three separate `{set:
  // {...}}` results would each clobber the last, not merge. That call site
  // was edited directly instead (see its own comment) to build `{ set: {
  // name, logo, color } }` up front, which needs no coercion at all.
  "workspace.update": {
    key: "workspace_update",
    renameIn: { id: "workspace" },
    coerceIn: {
      git: (value) => ({
        set: Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([k, v]) => [`git.${k}`, v])),
      }),
      worktrees: (value) => ({
        set: Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([k, v]) => [`worktrees.${k}`, v])),
      }),
      tasks: (value) => ({ set: { tasks: value } }),
    },
    wrapOut: "workspace",
  },

  // ── own HTTP surfaces ────────────────────────────────────────────────────
  "auth.getStatus": () => authApi.status(),
  "auth.login": (p) => authApi.login(s(p, "identifier"), s(p, "password")),
  "auth.logout": () => authApi.logout(),
  "auth.onboarding": (p) => authApi.onboarding(s(p, "name"), s(p, "email"), s(p, "password")),
  "session.get": () => authApi.session(),
  "password.change": (p) => authApi.changePassword(s(p, "current"), s(p, "next")),
  "file.create": (p) => fileApi.write(s(p, "path"), s(p, "content")),
  "file.delete": (p) => fileApi.remove(s(p, "path")),
  "file.diff": (p) => fileApi.diff(s(p, "path")),
  "file.list": (p) => fileApi.tree(s(p, "path"), p["recursive"] === true),
  "file.move": (p) => fileApi.move(s(p, "from"), s(p, "to")),
  "file.read": (p) => fileApi.read(s(p, "path")),
  "file.write": (p) => fileApi.write(s(p, "path"), s(p, "content")),

  // ── dormant: command missing from a live domain ─────────────────────────
  "activity.listEvents": null,
  "auth.verifyWaitlist": null,
  "chat.delete": null,
  "chat.findOrCreateDm": null,
  "chat.stop": null,
  "chat.toggleReaction": null,
  "chat.update": null,
  // The Go side has file.tree; the explorer returns a snapshot with
  // contexts. The difference is one of shape, not capability — a candidate
  // for an adapter, not a new implementation. Out of scope for this port.
  "file.changes": null,
  "file.explorer": null,
  "file.search": null,
  "session.updateProfile": null,
  "task.start": null,
  // `workspace.directory` (`workspace-directory.fetch.ts`, called by both
  // `AgentStore` and `WorkspaceStore` preload) has no Go counterpart at all
  // — there is no `workspace_directory`/`workspace_inventory`-shaped command
  // that returns agents+users combined the way Fractal's original did.
  // Before this entry existed, the call fell through `call()`'s loud "not
  // mapped" throw — caught locally by that one file's own try/catch (see its
  // "Falls back to raw fetch when the typed client schema is stale" comment)
  // and silently retried against `/api/workspaces/{id}/directory`, a REST
  // path Go's `httpapi` router (`internal/transport/httpapi/server.go`) never
  // registers — so the roster always resolved to `{ users: [], agents: [] }`,
  // quietly, no matter what. Declaring this `null` here doesn't change that
  // outcome (a dormant `query()` also resolves to empty for this call site),
  // but it makes the failure go through the one documented dormancy path
  // instead of a bespoke local one — and stops every agent-roster load from
  // needlessly throwing and 404ing first. The actual gap — no Go command
  // backs the agent roster the UI wants — is a task-12 escalation, not
  // something this map entry can fix: `agents_list` exists and is live, but
  // nothing wires it into `AgentStore` today.
  "workspace.directory": null,
  "workspace.addMember": null,
  "workspace.listMembers": null,
  "workspace.removeMember": null,
  "workspace.updateMember": null,

  // ── dormant: whole domain absent from Go ────────────────────────────────
  "artifact.delete": null,
  "artifact.getById": null,
  "artifact.list": null,
  "collection.createRecord": null,
  "collection.delete": null,
  "collection.deleteRecord": null,
  "collection.getById": null,
  "collection.getRecordById": null,
  "collection.list": null,
  "collection.listRecords": null,
  "collection.updateRecord": null,
  "goal.create": null,
  "goal.delete": null,
  "goal.getById": null,
  "goal.list": null,
  "goal.update": null,
  "instruction.create": null,
  "instruction.delete": null,
  "instruction.list": null,
  "instruction.update": null,
  "marketplace.getByName": null,
  "marketplace.list": null,
  "model.list": null,
  "model.set": null,
  "project.create": null,
  "project.delete": null,
  "project.getById": null,
  "project.list": null,
  "project.update": null,
  "skill.delete": null,
  "skill.install": null,
  "skill.list": null,
  "skill.update": null,
  "template.create": null,
  "template.delete": null,
  "template.list": null,
  "template.update": null,
  "token.regenerate": null,
  "toolset.delete": null,
  "toolset.getById": null,
  "toolset.getConfig": null,
  "toolset.updateConfig": null,
  "tunnel.getStatus": null,
  "tunnel.start": null,
  "tunnel.stop": null,
  "user.create": null,
  "user.delete": null,
  "user.list": null,
  "user.update": null,
  "view.delete": null,
  "view.executeAction": null,
  "view.getById": null,
  "view.list": null,
  "view.render": null,
};

/** The domains the Go backend does not have yet, whole. */
export const DORMANT_DOMAINS: ReadonlySet<string> = new Set([
  "artifact", "collection", "goal", "instruction", "marketplace", "model",
  "project", "skill", "template", "token", "toolset", "tunnel", "user", "view",
]);

/** Whether the whole domain is dormant — what the route shows as a panel. */
export function isDormant(feature: string): boolean {
  return DORMANT_DOMAINS.has(feature);
}
