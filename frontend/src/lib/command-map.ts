import type { CommandKey } from "./schema";
import * as fileApi from "./file";
import * as authApi from "./auth";

/**
 * One Fractal frontend call, resolved.
 *
 * - `CommandKey` — goes through the command registry (`client.invoke`).
 * - `HttpHandler` — goes through its own HTTP surface (`/api/auth`,
 *   `/api/file`), which sits outside the registry by backend decision.
 * - `null` — the Go side does not have this yet. See the dormancy contract.
 */
export type HttpHandler = (payload: Record<string, unknown>) => Promise<unknown>;
export type MapEntry = CommandKey | HttpHandler | null;

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
  "comment.list": "comments_list",
  "config.get": "config_get",
  "config.update": "config_update",
  "routine.create": "routines_create",
  "routine.delete": "routines_delete",
  "routine.fire": "routines_fire",
  "routine.getById": "routines_get",
  "routine.list": "routines_list",
  "routine.update": "routines_update",
  "task.delete": "tasks_delete",
  "task.getById": "tasks_get",
  "task.list": "tasks_list",
  "task.setStatus": "tasks_set-status",
  "task.update": "tasks_update",
  "theme.get": "themes_get",
  "theme.list": "themes_list",
  "todo.list": "todos_list",
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
