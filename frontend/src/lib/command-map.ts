import type { CommandKey } from "./schema";
import { toUiChat, toUiMessage } from "./chat-message";
import * as fileApi from "./file";
import * as fileExplorer from "./file-explorer";
import * as authApi from "./auth";
import { actionsOf, toSpec, type RenderedView } from "./view-spec";

/**
 * One AOS frontend call, resolved.
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
 * commands: many AOS inputs use a field name Go's command doesn't
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
   * Payload keys to rename before the call reaches Go — `{ jsName:
   * goName }`. Applied to the flattened `{...params, ...query, ...body}`
   * object, so it doesn't matter which of the three a field started in.
   * Runs *after* `coerceIn` (M1 of the final review: this comment said
   * "before" — the wrong one of the two; `aos-facade.ts`'s `call()` is
   * the ground truth: `applyRenameIn(applyCoerceIn(rawPayload,
   * descriptor.coerceIn), descriptor.renameIn)`, coerce innermost, so
   * first). `coerceIn`'s own doc comment below (keyed pre-`renameIn`) was
   * already correct — matched the code, contradicted this one. `useQuery`'s
   * cache key is computed from the *original* payload (what the calling
   * code actually passed, before either transform) — both are purely
   * outbound transport concerns, invisible to the ported code on both sides.
   */
  renameIn?: Record<string, string>;
  /**
   * Fields this mapping always sends, whatever the caller passed, keyed by
   * the name Go expects (so: after `renameIn`, which is when they are
   * applied).
   *
   * It exists for the mappings whose whole meaning is a constant — `task.
   * start` is `tasks_set-status` with `status: "in_progress"`, and there is
   * no version of "start" that means anything else. `coerceIn` cannot
   * express that: it only fires on keys the payload already carries, and a
   * caller saying "start this" does not send a status to correct.
   */
  fixedIn?: Record<string, unknown>;
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
  /**
   * Reshapes what Go answered before the ported code reads it, for a
   * response field whose *name* diverges rather than its nesting —
   * `wrapOut`'s counterpart on the value side.
   *
   * It receives the bare Go result (so: the array for a `*_list`, the
   * entity for a `*_get`) and runs *before* `wrapOut` nests it, which is
   * what lets one function serve both shapes of the same domain.
   *
   * Reach for this only when Go and the ported UI genuinely name the same
   * field differently. A field Go does not have at all is not a mapping
   * problem — that one belongs at the call site, where the UI can decide
   * what to render in its absence (see `goalPriorityConfig` in
   * `features/goal/presentation/consts/goal.ts`).
   */
  mapOut?: (data: unknown) => unknown;
}

export type MapEntry = CommandKey | CommandDescriptor | HttpHandler | null;

const s = (p: Record<string, unknown>, k: string): string => String(p[k] ?? "");

/**
 * The marker the composer puts on material it attached rather than the person
 * typed (`COMPOSER_PROMPT_PART_PREFIX` in `composer.helper.ts`, and the same
 * string the message renderer hides on).
 *
 * Declared here too rather than imported, so this map — which every feature
 * loads — does not pull a presentation helper and its transitive imports in
 * with it. The renderer and the composer already agree on the literal; a test
 * pins all three together.
 */
export const COMPOSER_CONTEXT_PREFIX = "[system-reminder]:";

/**
 * A chat, with every message translated for the ported components.
 *
 * The translation itself lives in `lib/chat-message.ts`, because a message
 * arrives through three doors — a chat read, the echo of a send, and the live
 * snapshot on the realtime channel — and only the first went through this
 * one. See that file for what it fixes and why.
 */
const withMessageMetadata = (chat: unknown): unknown => toUiChat(chat);

/**
 * A routine trigger, flattened onto the shape Go's input takes.
 *
 * Go stores a trigger nested (`{type, config: {cron}}`) and answers with it
 * nested, but `routine.TriggerInput` is flat (`{type, cron, namespace, event,
 * filters}`) — the shape a model can fill in without a nested object. The
 * interface, ported against the stored shape, sent the nested one on write:
 * `cron` arrived empty and every scheduled routine was refused with
 * AOS_ROUTINE_INVALID_CRON, while an activity trigger silently lost its
 * namespace, event and filters.
 *
 * `token` is not copied across: the webhook secret is minted by the daemon,
 * and a client that sends one is asking for a secret it chose itself.
 */
const flattenTriggers = (value: unknown): unknown => {
  if (!Array.isArray(value)) return { triggers: value };
  return {
    triggers: value.map((raw) => {
      if (!raw || typeof raw !== "object") return raw;
      const trigger = raw as Record<string, unknown>;
      const config = (trigger["config"] ?? {}) as Record<string, unknown>;
      const { config: _nested, ...flat } = trigger;
      const { token: _minted, ...carried } = config;
      return { ...flat, ...carried };
    }),
  };
};

/** One level in: `nested(p, "user", "name")` reads `p.user.name` as a string. */
const nested = (p: Record<string, unknown>, group: string, k: string): string => {
  const inner = p[group];
  if (inner === null || typeof inner !== "object") return "";
  return String((inner as Record<string, unknown>)[k] ?? "");
};

/** One-level `{a, b}` -> `{"prefix.a": a, "prefix.b": b}`, for a `coerceIn` that turns a nested UI object into `patch.Apply`'s dotted-path leaves. */
const dotted = (prefix: string, value: unknown): Record<string, unknown> =>
  Object.fromEntries(
    Object.entries(value as Record<string, unknown>).map(([k, v]) => [`${prefix}.${k}`, v]),
  );

/**
 * Go's `Goal` calls the deadline `dueAt`
 * (`internal/domain/goal/entity.go`); every ported goal screen reads
 * `goal.deadline` (`GoalHelper.formatDeadline`/`isOverdue`, the list row,
 * the detail form). Same field, two names — so the rename happens once
 * here rather than at each of the six read sites.
 *
 * Copies `dueAt` across instead of moving it: nothing reads `dueAt`
 * today, but leaving it means a future reader of the real Go name still
 * finds it, and `deadline` never disagrees with it.
 *
 * Note this is the read direction only. The write direction is
 * `renameIn: { deadline: "dueAt" }` on `goal.create`/`goal.update`, which
 * is why both appear on those entries.
 */
const withDeadline = (value: unknown): unknown => {
  const one = (goal: unknown) => {
    if (!goal || typeof goal !== "object") return goal;
    const record = goal as Record<string, unknown>;
    return record.dueAt === undefined ? record : { ...record, deadline: record.dueAt };
  };
  return Array.isArray(value) ? value.map(one) : one(value);
};

/**
 * Fills in the `stats.todos` breakdown the task detail screen reads off
 * every task, from the one aggregate Go actually keeps.
 *
 * `(\$id)/components/main/index.tsx` opens with `const todoStats =
 * task.stats.todos`, then sums `completed + in_progress + in_review +
 * todo` for the completion bar. Go's `Task` has no `stats` at all
 * (`internal/domain/task/entity.go`); what it has is `progress`
 * — `{completed, total}`, and its own doc says it "mirrors the todo
 * aggregate's count". So `task.stats` was `undefined` and reading
 * `.todos` off it threw before the page rendered a single element: the
 * task detail screen was unreachable for every task, which is the
 * "Cannot read properties of undefined (reading 'todos')" boundary.
 *
 * The projection is exact where the UI actually looks. `completed` and
 * the total both come straight from `progress`, so the percentage and
 * the all-done check are right. The split between the three unfinished
 * states is not data Go publishes per task — the daemon's own
 * `todo.Status` union is `pending|in_progress|blocked|finished|skipped`,
 * which doesn't even line up one-to-one with the four buckets this UI
 * declares — so the remainder all lands in `todo` rather than being
 * invented across the other two. The Todos widget on the same page reads
 * the real per-todo records through `todo.list` and is unaffected by
 * this.
 */
const withTaskStats = (value: unknown): unknown => {
  if (!value || typeof value !== "object") return value;
  const task = value as Record<string, unknown>;
  if (task.stats !== undefined) return task;

  const progress = (task.progress ?? {}) as { completed?: number; total?: number };
  const completed = progress.completed ?? 0;
  const total = progress.total ?? 0;

  return {
    ...task,
    stats: {
      todos: {
        completed,
        in_progress: 0,
        in_review: 0,
        todo: Math.max(0, total - completed),
      },
    },
    todos: task.todos ?? [],
    comments: task.comments ?? [],
  };
};

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
  // against the original source's own consumer (`agents.context.tsx` reads
  // `response.data?.agent`), not guessed; nothing in this app calls these
  // through the facade yet (no `agent` feature ported), so this was a
  // latent instance of the same bug `todo.getById` was, caught by the
  // full-map sweep the round-2 review asked for rather than by a live
  // failure.
  // No `wrapOut` on create and update, unlike `getById` below: their call
  // site (`agents.context.tsx`) reads `result?.data` and then `.id` on it —
  // the bare agent. Wrapping put the agent under `data.agent`, so `.id` was
  // always undefined and every successful save reported "Unable to save this
  // agent". `getById`'s caller really does read `data.agent`, which is why
  // that one keeps it.
  "agent.create": "agents_create",
  // `agents_delete` and `agents_update` both name the agent `id`
  // (internal/domain/agent/schema.go); every call site in the interface
  // sends `params: { agent }`, the name the ported UI uses. Without the
  // rename the payload reached Go with no `id` at all and the command was
  // refused by validation — the Agents settings page could neither save nor
  // delete.
  "agent.delete": { key: "agents_delete", renameIn: { agent: "id" } },
  // Found by the final-review sweep's corrected pattern (`client.<feature>.
  // <action>` alone, not requiring `.query`/`.mutate` on the same line) —
  // `agents.context.tsx`'s live `aos.client.agent.getById\n  .query(...)`
  // is a multi-line method chain the original adjacent-match sweep missed
  // entirely. `agents_get`'s `GetInput.ID` (`internal/domain/agent/
  // schema.go`) names the field `id`; the call site sends `params: {
  // agent: selectedAgentId }`. `agents_get` answers with a bare `*Agent`
  // (`internal/domain/agent/commands.go`), same as `agent.create`/`.update`
  // above; the call site reads `response.data?.agent`, confirming `wrapOut`.
  "agent.getById": { key: "agents_get", renameIn: { agent: "id" }, wrapOut: "agent" },
  // B3 of the final review: `AgentStore` (`features/agent/presentation/
  // stores/agent.store.ts`) used to source the roster from `workspace.
  // directory` — `null` in this map, no Go counterpart — while
  // `agents_list` sat right there, real and live, unused. `agents_list`
  // answers `ListOutput{agents: Agent[], total: int}`
  // (`internal/domain/agent/schema.go`) — already wrapped under `agents`,
  // so no `wrapOut` needed; the store reads `response.data?.agents`.
  "agent.list": "agents_list",
  "agent.update": { key: "agents_update", renameIn: { agent: "id" } },
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
  // Delete, rename and clear: the three actions every conversation row and
  // the composer have offered since they were ported, all of which did
  // nothing because this group published only list/get/create/send. See
  // internal/domain/chat/commands.go for the three commands that closed it.
  "chat.delete": "chats_delete",
  // `wrapOut`, the same as `chat.getById` below: Go answers a bare Chat, and
  // the ported code reads `result.data.chat`.
  "chat.update": { key: "chats_update", mapOut: withMessageMetadata, wrapOut: "chat" },
  // The composer's "clear context" used to call `chat.update` with
  // `{messages: []}` — a field Go's update does not have and must not have,
  // since a rename that can drop a transcript is a rename nobody can trust.
  // It is its own command now, and its own path here.
  "chat.clear": "chats_clear",
  "chat.create": { key: "chats_create", mapOut: withMessageMetadata, wrapOut: "chat" },
  "chat.getById": { key: "chats_get", mapOut: withMessageMetadata, wrapOut: "chat" },
  "chat.list": "chats_list",
  // task-12 (round 2): `use-chat-composer.ts`'s `sendMessage` calls this
  // with `body: { id: chat.id, message: <ChatMessage: {id, role,
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
  //
  // The reminder parts go to `context`, not into `text`. They used to be
  // joined into it — and they come *first*, since the composer attaches
  // instructions ahead of what was typed — so a workspace with a single
  // workspace-wide instruction produced a user message whose text began
  // "[system-reminder]: …". The renderer hides a text part that starts with
  // that prefix and renders nothing when none is left, so every message the
  // person sent appeared while it was an optimistic echo and vanished the
  // moment the daemon confirmed it. The model also read the reminder as part
  // of the user's own words.
  "chat.send": {
    key: "chats_send",
    coerceIn: {
      message: (value) => {
        const parts = (value as { parts?: Array<{ type: string; text?: string }> } | undefined)?.parts ?? [];
        const texts = parts
          .filter((part) => part.type === "text" && typeof part.text === "string")
          .map((part) => part.text as string);
        return {
          text: texts.filter((text) => !text.startsWith(COMPOSER_CONTEXT_PREFIX)).join("\n\n"),
          context: texts.filter((text) => text.startsWith(COMPOSER_CONTEXT_PREFIX)),
        };
      },
    },
    // The echo the composer swaps in for its optimistic message. Without
    // this it arrived raw, so the confirmed message lost the timestamp and
    // the day divider its echo had, and got them back only on the next
    // refetch.
    mapOut: (out) => {
      if (!out || typeof out !== "object") return out;
      const answered = out as Record<string, unknown>;
      if (!answered["message"]) return out;
      return { ...answered, message: toUiMessage(answered["message"]) };
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

  // Left without `wrapOut` on an earlier round for "genuine ambiguity" —
  // resolved now that `config.update` has a live `useForm` consumer.
  // `config_get`/`config_update` (`internal/domain/config/commands.go`)
  // both answer a bare `Config` value, not `{config: ...}`. The one live
  // consumer, `config.store.ts`'s `unwrapConfig`, was written defensively
  // to accept either shape (`"config" in data ? data.config : data`) — but
  // that tolerance is exactly the smell this map exists to remove: a
  // second call site with the same bare response and no such guard would
  // silently read `undefined`. Resolving it here removes the ambiguity for
  // every future caller: Go's response is bare, so no `wrapOut`.
  "config.get": "config_get",
  // Go's `UpdateInput` (`internal/domain/config/schema.go`) takes exactly
  // one field: `set`, a dotted-path map (`{"tunnel.enabled": true}`) —
  // `patch.Apply`'s contract, same one `workspace.update` already coerces
  // for (see that entry's own comment for the exact collision constraint
  // this mirrors). Every live caller instead sends a nested object shaped
  // like a slice of `Config` itself — `tunnel/index.tsx`'s `{tunnel:
  // {enabled}}`, `profile/index.tsx`'s `{region: {timezone, ...}}`,
  // `agents/index.tsx`'s `{agents: {models: {...}}}` — which Go's decoder
  // reads as an empty `Set` (no `set` key at all) and refuses with
  // `validate:"required"`. Every one of those three call sites' submits
  // was throwing before this entry existed; none of it was reachable by
  // playing with the UI, only by reading what actually left the browser.
  //
  // `dotted` flattens one level, matching each of these three fields'
  // real Go shape: `Tunnel`/`Region` are flat structs, and `Agents`' own
  // fields (`providers []Provider`, `models map[string]ModelRef`) are
  // leaves per `patch.Apply`'s own contract ("composite values are
  // leaves: a caller replaces the whole list") — so `agents.models` must
  // land as one dotted key holding the whole map, never flattened deeper
  // into `agents.models.default.provider`, which `patch.Paths` does not
  // recognise.
  //
  // `general`/`notifications` are deliberately NOT here: `general/
  // index.tsx`'s one submit sends both together, and `coerceIn`'s
  // per-field merge would collide on the `set` key exactly the way
  // `workspace.update`'s own comment describes for `.../profile` — fixed
  // at that one call site instead, building `{set: {...}}}` directly.
  "config.update": {
    key: "config_update",
    coerceIn: {
      tunnel: (value) => ({ set: dotted("tunnel", value) }),
      region: (value) => ({ set: dotted("region", value) }),
      agents: (value) => ({ set: dotted("agents", value) }),
    },
  },

  // Found by the final-review sweep's corrected pattern — `memories/
  // index.tsx`'s live `aos.client.memory.graph\n  .query(...)` is another
  // multi-line chain the original sweep missed. `memories_graph`'s
  // `GraphInput.Agent` (`internal/domain/memory/schema.go`) already names
  // its field `agent`, matching the call site's `query: { agent: ... } }`
  // — no `renameIn` needed. The output is a bare `Graph`
  // (`command.Command[GraphInput, Graph]`,
  // `internal/domain/memory/commands.go`), read straight off `response.
  // data` with no wrapping key, so no `wrapOut` either — but `Graph`'s own
  // shape (`{nodes: Node[], edges: Edge[], health, counts}`) diverges from
  // what the force-3d-graph render layer wants (`{nodes: {id, label,
  // group, val}[], links: {source, target, type}[]}`, per `memory.
  // interfaces.ts`'s `MemoryGraphSchema`) — Go's `edges` isn't `links`,
  // `Node.title` isn't `.label`, and there's no `.group`/`.val` at all.
  // Same class of gap as `theme.get`'s `fonts`: `wrapOut` only adds one
  // nesting level to a bare entity, it can't reshape fields several levels
  // in, so the reshape is a disclosed call-site fix (see that file's own
  // comment) rather than something expressible here.
  "memory.graph": "memories_graph",

  // The other four. Only `memory.graph` was mapped, so the desktop could draw
  // the shape of what an agent knew and could not read, write, revise or
  // retire a single memory of it — the core of the system, visible only as
  // dots on a graph.
  //
  // `memories_recall` answers `{memories, total, indexed}` and
  // `memories_reflect` a bare `Memory`, so only the latter needs wrapping.
  // `memory` is what both address a record by; the ported code calls it
  // `memory` too, so there is nothing to rename.
  "memory.list": "memories_recall",
  "memory.getById": { key: "memories_reflect", wrapOut: "memory" },
  "memory.create": { key: "memories_store", wrapOut: "memory" },
  // Not a delete. Forgetting deprecates and keeps the trace, with the reason
  // it stopped being true — which is why `reason` is required by Go and asked
  // for by the interface rather than defaulted to something polite.
  "memory.forget": "memories_forget",

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
  // `coerceIn` on triggers, and it is not cosmetic: every routine with a
  // schedule failed to save. The interface builds a trigger as
  // `{type, config: {cron}}` (the shape Go *stores* and answers with), while
  // `routine.TriggerInput` is flat — `{type, cron}` — so `Cron` arrived empty
  // and the daemon refused with AOS_ROUTINE_INVALID_CRON. An activity trigger
  // lost its namespace, event and filters the same way, silently. `token` is
  // dropped on the way in because Go mints the webhook secret itself.
  "routine.create": { key: "routines_create", renameIn: { prompt: "content" }, coerceIn: { triggers: flattenTriggers } },
  "routine.delete": { key: "routines_delete", renameIn: { routine: "id" } },
  // NOT `wrapOut: "run"`, on purpose. Go's `routines_fire` returns a
  // single bare `*Run` (`internal/domain/routine/commands.go`), but the
  // live consumer (`onSuccess` in `pages/($id)/index.tsx`, matching
  // the original's `routine-list-row.component.tsx`) reads
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
  "routine.update": { key: "routines_update", renameIn: { routine: "id", prompt: "content" }, coerceIn: { triggers: flattenTriggers } },
  // The runs of one routine. The routine page reads `routine.runs`, which
  // `routine.View` has never carried — the history panel was empty for every
  // routine — while `routines_runs` was a live command nothing called.
  "routine.runs": { key: "routines_runs", renameIn: { routine: "id" } },

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
  "task.getById": { key: "tasks_get", renameIn: { task: "id" }, wrapOut: "task", mapOut: withTaskStats },
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
  // match `ThemeSettings` by name, `semantic`/`semanticColors` is a
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
  // }` -> `workspace`, the field name the original UI still uses.
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
  // Accepts both spellings on purpose. `flattenArgs` merges params, query and
  // body one level deep, so a caller that groups its fields — `{user: {name,
  // email}, security: {password}}`, which is how the deleted second onboarding
  // screen sent them — leaves nothing under the flat keys, and this read three
  // empty strings and asked the daemon to create an account with a zero-length
  // password. Reading through the group as well costs a line and removes a way
  // to call this and get nothing.
  "auth.onboarding": (p) =>
    authApi.onboarding(
      s(p, "name") || nested(p, "user", "name"),
      s(p, "email") || nested(p, "user", "email"),
      s(p, "password") || nested(p, "security", "password"),
    ),
  "session.get": () => authApi.session(),
  "password.change": (p) => authApi.changePassword(s(p, "current"), s(p, "next")),
  "file.create": (p) => fileApi.write(s(p, "path"), s(p, "content")),
  "file.delete": (p) => fileApi.remove(s(p, "path")),
  "file.diff": (p) => fileApi.diff(s(p, "path")),
  // The three screens the port left unmapped, assembled from what the daemon
  // publishes — see lib/file-explorer.ts. Until this, the sidebar's file tree,
  // the Changes panel and the composer's @-mention picker all rendered empty,
  // with no error anywhere: a dormant call resolves to null rather than
  // failing, so all three looked like an empty workspace.
  "file.explorer": () => fileExplorer.explorer(),
  "file.changes": () => fileExplorer.changes(),
  "file.search": (p) => fileExplorer.search(s(p, "query"), Number(p["limit"]) || 24),
  // Through the explorer adapter, not `fileApi.tree` directly: the daemon
  // answers `{path, nodes}` and all three call sites read `.files` off a
  // `WorkspaceFile[]`, so this resolved to `undefined` at every one of them.
  "file.list": (p) => fileExplorer.list(s(p, "path"), p["recursive"] === true),
  "file.move": (p) => fileApi.move(s(p, "from"), s(p, "to")),
  "file.read": (p) => fileApi.read(s(p, "path")),
  "file.write": (p) => fileApi.write(s(p, "path"), s(p, "content")),

  // The catalogue of what a routine can react to. Go's `activity_events`
  // answers `{events, namespaces}` where each event carries `data` — the
  // payload keys a trigger filter can match on — while this side reads a bare
  // array of definitions whose filterable fields come from a JSON Schema's
  // `properties`. The keys are the whole of what the picker uses
  // (`ActivityEventHelper.getFilterableFields` reads nothing but
  // `Object.keys`), so they are rendered as a properties object rather than
  // as a schema promising types the publishers do not enforce.
  //
  // This was dormant, and it was the reason the routine editor could not
  // offer an activity trigger at all: Go has matched them since the routine
  // domain was written (`routine.Trigger.Matches`), the picker was fully
  // built, and the catalogue it reads answered nothing — so the one trigger
  // that reacts to the workspace was unreachable, and writing one meant
  // guessing a namespace and an event.
  "activity.listEvents": {
    key: "activity_events",
    mapOut: (raw) => {
      const answer = raw as { events?: Array<Record<string, unknown>> } | undefined;
      return (answer?.events ?? []).map((event) => ({
        namespace: event["namespace"],
        event: event["event"],
        title: event["title"],
        description: event["description"],
        schema: {
          properties: Object.fromEntries(
            ((event["data"] as string[] | undefined) ?? []).map((key) => [key, {}]),
          ),
        },
      }));
    },
  },
  // The approval channel (ADR-0007). Neither is in the agent's own registry —
  // an agent that could approve its own tool call would make the whole
  // mechanism decoration — so these are the interface's alone.
  //
  // They were unmapped until now, which meant the daemon published
  // `approval.request`, the chat rendered a tool stuck in `approval-requested`,
  // and nothing anywhere could answer it: the run waited out its deadline and
  // was denied by timeout, every time.
  "approval.list": "approvals_list",
  "approval.decide": "approvals_decide",

  // Keeping the installation current. `update_check` never downloads;
  // `update_download` verifies the signature before a single asset is
  // fetched and stages nothing on a mismatch; `update_apply` swaps and rolls
  // back on failure. All four existed and none was reachable — a desktop
  // application that could not tell you a new version was out, let alone
  // install it.
  "update.status": "update_status",
  "update.check": "update_check",
  "update.download": "update_download",
  "update.apply": "update_apply",

  // The execution queue. `jobs_list`/`jobs_stats` are what makes a turn that
  // is queued, running, retrying or dead visible at all; without them the
  // window showed nothing between "asked" and "answered".
  "job.list": "jobs_list",
  "job.stats": "jobs_stats",
  "job.getById": { key: "jobs_get", renameIn: { job: "id" } },
  "job.recover": "jobs_recover",
  "job.purge": "jobs_purge",

  // What the daemon says about itself. Only `status` and `restart` are
  // mapped: `start` asks a running daemon to start itself, and `stop` would
  // have the window cut the connection it is speaking over — neither is an
  // action this surface can offer honestly. Supervision belongs to whatever
  // launched the daemon (`aos gateway`, or the desktop's own supervisor).
  "gateway.status": "gateway_status",
  "gateway.restart": "gateway_restart",

  "auth.verifyWaitlist": null,
  // `chat.findOrCreateDm`, `chat.stop` and `chat.toggleReaction` are the
  // three of the five that are still dormant, and each for its own reason:
  // a direct-message lookup needs a stable two-party key Go's `chats_create`
  // does not derive yet, stopping a run needs the job queue to be able to
  // cancel one mid-turn, and a reaction is a per-message record the chat
  // collection has no shape for. None of the three can be composed out of
  // what exists, which is what separates them from `chat.delete`/`.update`
  // (below) — those were missing commands, and now are not.
  // Stays dormant deliberately, and is not a broken button: Go has no
  // find-or-create for chats, and `open-chat-tab.helper.ts` already does the
  // two steps itself (`chats_list` filtered by kind, then `chats_create`).
  // Mapping this onto `chats_create` would put a second, competing
  // implementation behind a name nothing calls.
  "chat.findOrCreateDm": null,
  // Both were dormant because Go had no such command. It has both now, so
  // the Stop button ends the turn instead of answering "no active run was
  // found to stop" while the agent kept working, and a reaction is stored on
  // the message the interface has always drawn one on.
  "chat.stop": "chats_stop",
  // `actor` is dropped: who is reacting comes from the identity of the call.
  // A caller that could name the actor could react as somebody else.
  "chat.toggleReaction": {
    key: "chats_react",
    renameIn: { messageId: "message" },
    coerceIn: { actor: () => ({}) },
    mapOut: withMessageMetadata,
    wrapOut: "chat",
  },

  // Not dormant: `POST /api/auth/profile` has existed since the identity
  // surface was built (`internal/transport/authapi`), and `lib/auth.ts`
  // already wraps it. Declaring it null made the profile screen's save
  // report "the session domain does not exist in the Go backend yet".
  "session.updateProfile": (p) => authApi.updateProfile(s(p, "name"), s(p, "email")),
  // Was dormant, and did not need to be: starting a task is moving it to
  // `in_progress`, which `tasks_set-status` has always done. The Start button
  // on the task page called this, got the dormant-domain error, and showed
  // "Failed to start task" — every time.
  //
  // `fixedIn` rather than `coerceIn`: the status is not a correction to
  // something the caller sent, it is the whole meaning of this mapping, and
  // `coerceIn` only fires on keys the payload already has.
  //
  // The caller also sends `delegate: true`, which `tasks_set-status` has no
  // field for and the daemon ignores. Starting the task is what the button
  // says it does; delegating it is a separate capability that does not exist
  // yet, and is listed as such rather than half-sent here.
  "task.start": {
    key: "tasks_set-status",
    renameIn: { task: "id" },
    fixedIn: { status: "in_progress" },
  },
  // `workspace.directory` stays unmapped, and is now unused: there is no
  // single Go command that answers agents+users the way the original's
  // server-side `getDirectory()` did, and there does not need to be.
  // `workspace-directory.fetch.ts` composes the roster from the two halves
  // the daemon does publish — `agents_list` (live) and `/api/auth/users` —
  // which is the same answer assembled one layer up.
  //
  // What this replaced: the roster resolved to `{ users: [], agents: [] }` on
  // every load, with no error anywhere, so the sidebar's Team tab, the task
  // assignee picker, the chat participant list and every avatar in the
  // interface rendered empty. It was not failing to load; it was loading
  // nothing.
  "workspace.directory": null,
  "workspace.addMember": null,
  "workspace.listMembers": null,
  "workspace.removeMember": null,
  "workspace.updateMember": null,

  // ── dormant: whole domain absent from Go ────────────────────────────────
  // task-10: the `collection` domain is lit — `internal/domain/collection/
  // commands.go` registers all nine. Every `collections_*` field name
  // below comes from that file's own Input structs, not guessed.
  //
  // `collections_get`/`collections_delete` name the collection `id`; the
  // ported UI calls that param `collection` on both
  // (`.../pages/($id)/index.tsx`'s `getById`,
  // `plugin-detail-section.component.tsx`'s `deleteCollection`) — hence
  // `renameIn: { collection: "id" }` on each. The four `records-*`
  // commands take `collection` (the parent) and `id` (the record) — the UI
  // already calls the parent `collection` (matches, no rename) but calls
  // the record `record` on `getRecordById`/`updateRecord`/`deleteRecord`
  // (`.../records/($record)/index.tsx`), hence `renameIn: { record: "id" }`
  // on those three; `records-create` has no record id to rename at all.
  //
  // `collections_get` answers bare (`*Collection`); the loader reads
  // `collection.data.collection` — `wrapOut: "collection"`. `collections_
  // create` has no live caller yet — the "add collection" flow was never
  // ported — so nothing here is asserted for it beyond the field names.
  "collection.createRecord": "collections_records-create",
  "collection.delete": { key: "collections_delete", renameIn: { collection: "id" } },
  "collection.deleteRecord": { key: "collections_records-delete", renameIn: { record: "id" } },
  "collection.getById": { key: "collections_get", renameIn: { collection: "id" }, wrapOut: "collection" },
  "collection.getRecordById": { key: "collections_records-get", renameIn: { record: "id" } },
  "collection.list": "collections_list",
  // `collection.listRecords` is disclosed, not fixed: `records-list`
  // answers `{records: Record[], total}` (`RecordsListOutput`,
  // `internal/domain/collection/commands.go`); the live call site
  // (`.../pages/($id)/index.tsx`) reads `records.data ?? []` — the bare
  // array itself, not a field nested one level in. `wrapOut` only adds a
  // nesting level to a bare entity; it cannot strip one that is already
  // there, so this is a call-site fix (read `records.data?.records`)
  // for whoever un-dormants this page next, the same class of gap
  // `memory.graph`'s own comment above describes.
  "collection.listRecords": "collections_records-list",
  "collection.updateRecord": { key: "collections_records-update", renameIn: { record: "id" } },
  // `model.list` is lit: `internal/domain/model/commands.go` registers
  // `models_list`, which asks each connected provider what it serves
  // instead of answering from a list inside the build. Go's `ListOutput`
  // is `{providers: [{id, models, error}], total}` — a shape the original
  // never had, because the original's `models()` was per-adapter and
  // mostly hardcoded (`_extracted/v401/.../adapters/*.adapter.ts`: only
  // OpenRouter actually fetched). No `wrapOut`/`mapOut`: the one consumer
  // (`model-provider.service.ts`) reads `.providers` directly and merges
  // it onto the static catalog, which still owns what a provider *is*
  // (name, description, auth mode) — none of which any provider publishes.
  "model.list": "models_list",
  // Still dormant: connecting a provider and choosing its models is
  // `config.update` on `agents.providers`/`agents.models` here, not a
  // command of its own. `model-provider.service.ts` is that seam.
  "model.set": null,
  // task-10: the `skill` domain is lit — `internal/domain/skill/
  // commands.go` registers list/install/create/update/delete.
  //
  // `skill.delete`/`skill.update` name the skill `skill` on the ported UI
  // (`marketplace/[name]/inner.tsx`'s `deletePlugin`/`updatePlugin`, both
  // `{ params: { skill: installedSkill.id } }`); Go's `DeleteInput`/
  // `UpdateInput` name it `id` — `renameIn: { skill: "id" }` on both.
  // `updatePlugin` sends `{ active }`, which is `UpdateInput.Active`
  // verbatim — no further coercion needed, and this is the whole reason
  // `skills_update` exists at all rather than only `skills_delete`/
  // `skills_install`: the ported UI's only "update" is this on/off toggle,
  // which is also the only field `skill.Installer.Update` lets change in
  // place (see its own doc comment) — the two were sized to match, not
  // discovered to.
  //
  // `skill.install`'s live caller (`marketplace-install-button.component.
  // tsx`) sends `{ source: "aos/registry", skill: pluginName }` — a
  // registry address plus a name, for installing a *named* package from a
  // marketplace. `InstallRequest.Source` (`internal/domain/skill/
  // commands.go`) is a location a `Fetcher` can actually read from — this
  // build's only `Fetcher` is `skillfetch.Local`, a local directory, which
  // "aos/registry" is not and never resolves to; `skill` (the plugin name)
  // has no field to land in at all and is dropped, the same way any other
  // field Go's decoder does not recognise is. Installing a marketplace
  // skill by name therefore fails today with a real, honest fetch error
  // instead of a silent no-op — disclosed here because there is no
  // registry `Fetcher` in this build for `renameIn`/`coerceIn` to bridge
  // to, not because the mapping is wrong.
  //
  // `skill.list` answers `{skills: Skill[], total}` (`ListOutput`); every
  // live caller (`use-chat-composer.ts`, both marketplace pages) reads
  // `.data?.skills` directly — matches with no `wrapOut`. Its own `query:
  // "..."` search field has no Go counterpart (`ListInput` takes only
  // `_reasoning`) and is dropped, the same class of gap as `skill.install`'s
  // `skill` field.
  "skill.delete": { key: "skills_delete", renameIn: { skill: "id" } },
  "skill.install": "skills_install",
  "skill.list": "skills_list",
  "skill.update": { key: "skills_update", renameIn: { skill: "id" } },
  "token.regenerate": null,
  // task-10: the `toolset` domain is lit — `internal/domain/toolset/
  // commands.go` registers get/get-config/update-config/delete for the UI
  // (list/call are agent/CLI-only, per the closed table).
  //
  // Every live caller (`plugin-inventory-item-sheet.component.tsx`,
  // `plugin-detail-section.component.tsx`) names the toolset `toolset` —
  // `{ params: { toolset: toolsetId } }` on all four; Go names it `id` —
  // `renameIn: { toolset: "id" }` on all four.
  //
  // `getById`/`getConfig` both answer bare (`*Toolset`,
  // `toolsets_get`/`toolsets_get-config` share one handler —
  // `internal/domain/toolset/commands.go`'s own comment on `get-config`
  // says so). `detailQuery` reads `detailQuery.data.toolset` (`"toolset"
  // in detailQuery.data`) — `wrapOut: "toolset"` on `getById`.
  // `getConfig`'s own reads (`configQuery.data?.requirements`,
  // `.connectionType`) name fields `Toolset` does not have at all — no
  // "requirements" list, no "connectionType" alongside `Type` — a shape
  // this rebuild's toolset has no equivalent for, disclosed rather than
  // guessed at with a `wrapOut` that cannot invent fields that are not
  // there.
  //
  // `updateConfig`'s live caller sends `{ values: <env map> }` — a UI name
  // for what `UpdateConfigInput.Env` is — `coerceIn` turns it into `{env:
  // value}`, merged over the renamed `id`.
  "toolset.delete": { key: "toolsets_delete", renameIn: { toolset: "id" } },
  "toolset.getById": { key: "toolsets_get", renameIn: { toolset: "id" }, wrapOut: "toolset" },
  "toolset.getConfig": { key: "toolsets_get-config", renameIn: { toolset: "id" } },
  "toolset.updateConfig": {
    key: "toolsets_update-config",
    renameIn: { toolset: "id" },
    coerceIn: { values: (value) => ({ env: value }) },
  },
  // `user.list` is the roster of accounts on this installation. It goes
  // through its own HTTP surface for the same reason `auth.*` and `file.*`
  // do: identity sits outside the command registry by backend decision (see
  // internal/transport/authapi's package doc), so there is no `users_list`
  // command key to map to.
  //
  // The three writes stay dormant. The domain has them
  // (auth.Service.SetRole/Delete, and Onboarding for the first account), but
  // no surface publishes them yet, and inventing a client-side compound out
  // of endpoints that do not exist would be worse than saying so — a
  // DormantGate on the section is a screen that explains itself.
  "user.create": null,
  "user.delete": null,
  "user.list": () => authApi.users(),
  "user.update": null,
  // task-10: the `view` domain is lit — `internal/domain/view/
  // commands.go` registers list/get/render/execute-action/delete for the
  // UI (create/components/scaffold are agent/CLI-only, per the closed
  // table).
  //
  // `getById`/`render`/`delete` all name the view `view` on the ported UI
  // (`view/presentation/pages/($view)/index.tsx`'s loader,
  // `plugin-detail-section.component.tsx`'s `deleteView`); Go names it
  // `id` on all three — `renameIn: { view: "id" }`.
  //
  // `getById` answers bare (`*View`); the loader reads
  // `viewResult.data?.view` — `wrapOut: "view"`.
  //
  // `render`'s live caller reads `renderResult.data?.result` as a
  // `ViewRenderResult` (a `json-render` `Spec`) — the original's render
  // endpoint evaluated a `json-render` document. This rebuild's
  // `views_render` (`Service.Render`, `internal/domain/view/service.go`)
  // answers `Rendered{view, records, renderedAt}` instead: the declared
  // tree plus the rows it binds to, not a pre-evaluated spec — a different
  // rendering model, not a renamed field, so there is no `result` for
  // `wrapOut` to nest anything under. Disclosed rather than forced: the
  // page that reads `ViewDataHelper.getSpec(renderResult)` needs its own
  // adapter from `{view, records}` to a `Spec` when this page is un-
  // dormanted, which is a UI decision this map cannot make.
  //
  // `executeAction`'s live caller sends `{ params: { view, actionId },
  // body: { params } }` and reads `response.data?.result`.
  // `ExecuteActionInput` (same file) names the view `id`, the action
  // `label`, and its arguments `input` — `renameIn: { view: "id", actionId:
  // "label" }` plus `coerceIn` folding the UI's own `params` object into
  // `input`. The response is `wrapOut: "result"`: `views_execute-action`
  // answers whatever the dispatched command returns, bare
  // (`json.RawMessage`, `Service.ExecuteAction`'s own signature) — nesting
  // it under `result` matches the read, but the fields the UI reads *off*
  // that result (`success`, `updates`) are the original's own action
  // protocol, not something every registered command answers; whether a
  // given action's result actually has them depends on which command the
  // view's own tree names, which is a call-site concern this map cannot
  // resolve generically.
  //
  // `view.list` has no live caller yet — no page lists views directly —
  // so it is mapped on field names alone.
  "view.delete": { key: "views_delete", renameIn: { view: "id" } },
  "view.executeAction": {
    key: "views_execute-action",
    renameIn: { view: "id", actionId: "label" },
    coerceIn: { params: (value) => ({ input: value }) },
    wrapOut: "result",
  },
  "view.getById": { key: "views_get", renameIn: { view: "id" }, wrapOut: "view" },
  "view.list": "views_list",
  // Go answers `{view, records, renderedAt}` — the composition and the data,
  // separately, with every binding still unresolved — and the screen renders a
  // flat `@json-render` spec. Nothing translated one into the other, so
  // `ViewDataHelper.getSpec` read a `spec` field that never arrived and every
  // view, always, rendered the renderer's "no renderable spec" panel. The
  // translation is `view-spec.ts`; the actions come with it, because Go keeps
  // them on the nodes that offer them and the page looks for them on the view.
  "view.render": {
    key: "views_render",
    renameIn: { view: "id" },
    mapOut: (raw) => {
      const rendered = raw as RenderedView | undefined;
      return {
        view: { ...(rendered?.view ?? {}), actions: actionsOf(rendered?.view?.tree) },
        spec: toSpec(rendered),
        records: rendered?.records ?? [],
        renderedAt: rendered?.renderedAt,
      };
    },
  },

  // The seven Phase 8 domains declared alongside the ecosystem core — see
  // docs/08 - Entrega/Roteiro de Fases.md's "Fora do núcleo, declarado".
  // The five/six-entry shape (list/getById/create/update/delete, +render for
  // template) mirrors view.*/toolset.* above; the specific paths kept here
  // (artifact.delete/getById/list, goal's five, instruction's five,
  // marketplace.getByName/list, project's five, template's five,
  // tunnel.getStatus/start/stop) are exactly the call sites the ported
  // frontend already had declared `null` for — real field names, confirmed
  // by that dormancy sweep, not guessed from the Go schema. The remaining
  // entries per domain (artifact's create/update/setPassword, template's
  // render, marketplace.install) have no live caller yet, the same
  // "no live caller" status collection.createRecord and skill's unused
  // fields above already carry.
  // `artifacts_list` (`internal/domain/artifact/service.go`'s `List`)
  // answers a bare `[]Artifact`, not `{artifacts: [...]}` — the live
  // consumer, `artifact.store.ts`'s `preload`/`refresh`, reads
  // `response.data?.artifacts`. Without `wrapOut` here, that read was always
  // `undefined` off a bare array, so `ArtifactStore.items` stayed empty no
  // matter how many artifacts existed — found while wiring the sidebar's
  // "Surfaces" group up to a real backend, not by a live failure report.
  "artifact.list": { key: "artifacts_list", wrapOut: "artifacts" },
  // `artifacts_get` also answers bare, same as `artifacts_list` above.
  // `plugin-detail-section.component.tsx`'s live caller reads
  // `result.data?.artifact` plus a *sibling* `result.data?.urls` — that
  // second read is itself wrong (this rebuild nests `urls` inside the
  // artifact, not beside it; see `Artifact.URLs`, `entity.go`) and was
  // fixed at that call site instead, since `wrapOut` only adds one nesting
  // level and cannot invent a sibling key that was never on the wire.
  "artifact.getById": { key: "artifacts_get", renameIn: { artifact: "id" }, wrapOut: "artifact" },
  "artifact.create": "artifacts_create",
  "artifact.update": { key: "artifacts_update", renameIn: { artifact: "id" } },
  "artifact.setPassword": { key: "artifacts_set-password", renameIn: { artifact: "id" } },
  "artifact.delete": { key: "artifacts_delete", renameIn: { artifact: "id" } },

  // `goals_list` answers a bare []Goal (internal/domain/goal/service.go's
  // List) — same bug class as artifacts_list above. Three live readers
  // (workspace home, goal's own (main) page, and the Goals tab inside a
  // project's detail page) all read response.data?.goals and got an empty
  // list regardless of what actually existed.
  "goal.list": { key: "goals_list", wrapOut: "goals", mapOut: withDeadline },
  "goal.getById": { key: "goals_get", renameIn: { goal: "id" }, wrapOut: "goal", mapOut: withDeadline },
  // goals_create answers bare too; the live caller (goal/($id)/index.tsx)
  // reads result.data?.goal?.id to navigate to the new goal after creating
  // it — always undefined before this, so "create" silently never
  // navigated anywhere. goal.update carries wrapOut for the same reason
  // goal.getById already did, even though its one live caller today only
  // checks result?.error and does not read the body.
  "goal.create": { key: "goals_create", renameIn: { deadline: "dueAt" }, wrapOut: "goal", mapOut: withDeadline },
  "goal.update": { key: "goals_update", renameIn: { goal: "id", deadline: "dueAt" }, wrapOut: "goal", mapOut: withDeadline },
  "goal.delete": { key: "goals_delete", renameIn: { goal: "id" } },

  "instruction.list": "instructions_list",
  // instructions_get/-create/-update all answer bare too. The live caller
  // (instructions.context.tsx) reads response.data?.instruction for all
  // three — get, create and update — so every one of them silently failed
  // to ever show or persist a loaded/created/edited instruction.
  "instruction.getById": { key: "instructions_get", renameIn: { instruction: "id" }, wrapOut: "instruction" },
  "instruction.create": { key: "instructions_create", wrapOut: "instruction" },
  "instruction.update": { key: "instructions_update", renameIn: { instruction: "id" }, wrapOut: "instruction" },
  "instruction.delete": { key: "instructions_delete", renameIn: { instruction: "id" } },

  // `marketplace.list` is the ported UI's name for a search/browse call —
  // maps to `marketplace_discovery`, not a literal `marketplace_list` (no
  // such command; discovery is the list-equivalent). `getByName` maps to
  // `marketplace_get`, which takes `source` — `renameIn: { name: "source" }`
  // is a guess at the UI's own param name, not yet confirmed against the
  // call site; `install` has no live caller yet.
  // The marketplace screens read `.items` off the list and `.skill` off the
  // detail, and sent `query`/`category`/`page`/`pageSize`. Go answers a bare
  // `[]Listing` and a bare `*Listing`, and its search takes `text`/`tag` —
  // so the list rendered nothing (`data.items` was undefined on an array)
  // and the search box filtered nothing. `page`/`pageSize` have no Go
  // counterpart at all and are dropped rather than sent to be ignored.
  "marketplace.list": {
    key: "marketplace_discovery",
    renameIn: { query: "text", category: "tag" },
    coerceIn: { page: () => ({}), pageSize: () => ({}) },
    wrapOut: "items",
  },
  "marketplace.getByName": { key: "marketplace_get", renameIn: { name: "source" }, wrapOut: "skill" },
  "marketplace.install": "marketplace_install",

  // Same bug class as goal.* just above: projects_list/-get/-create all
  // answer bare (internal/domain/project/service.go). Three live readers
  // expect wrapped responses — workspace home and project's own (main)
  // page read response.data?.projects; project/($id)/index.tsx's loader
  // reads result.data?.project (so opening any project 404'd, since that
  // read was always undefined) and its create handler reads
  // result.data?.project?.id to navigate to the new project afterward.
  "project.list": { key: "projects_list", wrapOut: "projects" },
  "project.getById": { key: "projects_get", renameIn: { project: "id" }, wrapOut: "project" },
  "project.create": { key: "projects_create", wrapOut: "project" },
  "project.update": { key: "projects_update", renameIn: { project: "id" }, wrapOut: "project" },
  "project.delete": { key: "projects_delete", renameIn: { project: "id" } },

  "template.list": "templates_list",
  "template.getById": { key: "templates_get", renameIn: { template: "id" } },
  "template.create": "templates_create",
  // Same shape as its four siblings above and below: `templates_update`'s
  // own `UpdateInput.ID` is `id`, and `templates.context.tsx` sends
  // `params: { template }`. The rename was on every other template command
  // and missing only here, so saving a template failed validation while
  // reading, deleting and rendering one worked.
  "template.update": { key: "templates_update", renameIn: { template: "id" } },
  "template.delete": { key: "templates_delete", renameIn: { template: "id" } },
  "template.render": { key: "templates_render", renameIn: { template: "id" } },

  // Registry: false keeps these out of the agent's own tool list only — the
  // HTTP surface a settings panel calls still reaches them (see
  // internal/domain/tunnel/commands.go's own doc on that distinction).
  "tunnel.getStatus": "tunnel_status",
  "tunnel.start": "tunnel_start",
  "tunnel.stop": "tunnel_stop",
};

/** The domains the Go backend does not have yet, whole. */
export const DORMANT_DOMAINS: ReadonlySet<string> = new Set([
  "token", "user",
]);

/** Whether the whole domain is dormant — what the route shows as a panel. */
export function isDormant(feature: string): boolean {
  return DORMANT_DOMAINS.has(feature);
}

/**
 * Whether one specific call path is dormant (`null` in `COMMAND_MAP`).
 *
 * `isDormant`/`DORMANT_DOMAINS` only sees whole-domain absence — every
 * `feature.*` path missing, the way `collection` or `instruction` are.
 * C6 of the final review needed a narrower check: `WorkspaceMembersSection`
 * lives in the very much *not* dormant `workspace` domain
 * (`workspace.create`/`.update`/`.list` are real), but every command that
 * section actually calls — `workspace.addMember`/`.listMembers`/
 * `.removeMember`/`.updateMember`, `user.list` — is individually `null`.
 * `DormantGate`'s `commands` prop uses this to gate a section on its own
 * specific dependencies instead of its domain's.
 */
export function isCommandDormant(path: string): boolean {
  return COMMAND_MAP[path] === null;
}
