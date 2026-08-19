import type { CommandKey } from "./schema";
import * as fileApi from "./file";
import * as authApi from "./auth";

/**
 * One Fractal frontend call, resolved.
 *
 * - `CommandKey` — goes through the command registry (`client.invoke`).
 * - `CommandDescriptor` — same, plus the two adaptations a Go command
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
 * This exists because a plain string couldn't say two things command-map.ts
 * needed to say, discovered only once real ported pages exercised real Go
 * commands: many Fractal inputs use a field name Go's command doesn't
 * (`task` where Go wants `id`), and many Go commands answer with a bare
 * entity where the ported code expects it nested under a domain key
 * (`tasks_get` returns the task itself; `client.task.getById.query(...)`
 * reads `result.data.task`). Fixing either by hand-editing every call site
 * would mean 25 more features silently reinventing the same fix, or
 * forgetting to.
 */
export interface CommandDescriptor {
  /** The registry key `client.invoke` actually calls. */
  key: CommandKey;
  /**
   * Payload keys to rename before the call reaches Go — `{ fractalName:
   * goName }`. Applied to the flattened `{...params, ...query, ...body}`
   * object, so it doesn't matter which of the three a field started in.
   * `useQuery`'s cache key is computed from the *unrenamed* payload (what
   * the calling code actually passed) — renaming is purely an outbound
   * transport concern, invisible to the ported code on both sides.
   */
  renameIn?: Record<string, string>;
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
  "activity.markAsRead": "activity_read",
  "agent.create": "agents_create",
  "agent.delete": "agents_delete",
  "agent.update": "agents_update",
  "chat.create": "chats_create",
  "chat.getById": "chats_get",
  "chat.list": "chats_list",
  "chat.send": "chats_send",

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
  "routine.create": "routines_create",
  "routine.delete": "routines_delete",
  "routine.fire": "routines_fire",
  "routine.getById": "routines_get",
  "routine.list": "routines_list",
  "routine.update": "routines_update",

  // Every `tasks_*` command that names one task takes `id`
  // (`internal/domain/task/schema.go`'s `GetInput`/`UpdateInput`/
  // `SetStatusInput`/`DeleteInput`, all `ID string json:"id"`) — the
  // ported code uniformly calls it `task`. `tasks_get`/`tasks_create`/
  // `tasks_update` also answer with the bare task (`*View`), not `{task:
  // ...}`; `tasks_set-status` already answers `{task, from, to}` and
  // needs no `wrapOut`. `tasks_delete`'s `DeleteOutput{id, ...}` is read
  // by no ported call site, so it stays unwrapped too.
  "task.create": { key: "tasks_create", wrapOut: "task" },
  "task.delete": { key: "tasks_delete", renameIn: { task: "id" } },
  "task.getById": { key: "tasks_get", renameIn: { task: "id" }, wrapOut: "task" },
  "task.list": "tasks_list",
  "task.setStatus": { key: "tasks_set-status", renameIn: { task: "id" } },
  "task.update": { key: "tasks_update", renameIn: { task: "id" }, wrapOut: "task" },

  "theme.get": "themes_get",
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
  "todo.getById": { key: "todos_get", renameIn: { taskId: "task" } },
  "todo.list": { key: "todos_list", renameIn: { taskId: "task" } },
  "todo.setStatus": { key: "todos_set-status", renameIn: { taskId: "task" } },
  "todo.update": { key: "todos_update", renameIn: { taskId: "task", description: "title", instructions: "content" }, wrapOut: "todo" },

  "workspace.create": "workspace_create",
  "workspace.delete": "workspace_delete",
  "workspace.list": "workspace_list",

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
