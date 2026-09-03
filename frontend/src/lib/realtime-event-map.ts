import { toLiveUiMessage } from "./chat-message";
import type { RealtimeEvent } from "./realtime";

/**
 * One AOS realtime subscription, resolved onto the daemon's real
 * channel — the same shape `command-map.ts` uses for calls, one layer up.
 *
 * The original's own frontend (and the copied `app/lib/realtime.ts` this
 * replaces) named its events `chat:refresh`, `files:changed`, and so on.
 * The daemon's actual channel (`internal/transport/realtime/hub.go`) speaks
 * a different vocabulary — `chat.delta`, `chat.done`, `activity`, `task.
 * changed`, `approval.request`, `collection.changed` — with different
 * payload shapes. Every `useRealtime("original:name", cb)` call site in the
 * ported tree stays untouched; this is the one place that translates.
 *
 * - `string` — subscribe to the named daemon event, payload passed through
 *   unchanged.
 * - `RealtimeEventDescriptor` — subscribe to the named daemon event, and
 *   reshape its `data` into the payload shape the ported callback expects.
 * - `null` — the daemon has no event for this original name. Declared, not
 *   omitted, for the same reason `command-map.ts`'s dormant entries are: a
 *   missing key here reads as "forgot to translate this one", not "checked,
 *   and there's genuinely nothing on the other end".
 */
export interface RealtimeEventDescriptor {
  /**
   * The daemon's event type (`internal/transport/realtime/hub.go`'s
   * `Event.Type`), or several of them when one original event is fed by
   * more than one daemon event. `chat:refresh` is the case that needs it:
   * the daemon reports an answer being written and an answer being
   * finished as two different events, and the original's single callback
   * handles both — it reads `payload.message` when there is one and
   * refetches when there is not.
   */
  type: string | string[];
  /**
   * Reshapes the daemon event into the payload the ported callback expects.
   * Receives the whole event (`data` is most of what's needed, but a few
   * translations — `activity:created`'s `workspaceId` — read `event.
   * workspace` too). Omit when the daemon's `data` already has the shape
   * the callback wants unchanged.
   */
  adapt?: (event: RealtimeEvent) => unknown;
}

export type RealtimeMapEntry = string | RealtimeEventDescriptor | null;

/**
 * The five event names the ported tree actually subscribes to, found by
 * grepping every `useRealtime("...", ...)` call site under `src/features`
 * (`hooks/use-realtime.ts` is the only hook name in play — AOS's own
 * `lib/realtime.ts` export sharing that name is a different mechanism, see
 * that file's own comment). A call site using a sixth name would resolve to
 * `undefined` here and get a console warning at subscribe time — the same
 * "fail loud, not silent" shape `command-map.ts` uses, scaled down: a
 * realtime subscription that never fires has no error state to land in the
 * way a failed call does, so silence here is worse, not the same.
 */
export const REALTIME_EVENT_MAP: Record<string, RealtimeMapEntry> = {
  // `use-chat.ts` (`payload.chatId`, optional `payload.message`/`.replace`)
  // and `layout/index.tsx` (`if (!payload.message) router.invalidate()`).
  // The daemon has no per-message "refresh" signal — only `chat.done`, at
  // the end of a whole turn (`internal/app/runtime.go`'s `publisher.
  // ChatDone`, `Data: {chat, usage}`). That means `payload.message` is
  // always absent through this translation: `use-chat.ts` always takes its
  // `chatQuery.refetch()` branch instead of the cheaper local-snapshot
  // branch, and `layout/index.tsx` always calls `router.invalidate()`.
  // Correct, not free — a full refetch per completed turn instead of a
  // patched-in snapshot per message — and the closest daemon signal there
  // is; there is no message-level event to translate onto instead.
  // `chat.message` carries the answer as it is written — text so far, plus
  // every tool call and result the turn has made (`internal/runtime/
  // session`'s `liveAnswer`). It is the same shape a stored message has and
  // the same id the answer will be stored under, so the ported callback's
  // `payload.message` branch patches it straight into the timeline and the
  // finished answer later replaces it rather than appearing beside it.
  //
  // `chat.done` stays alongside it as the reconciliation point: it carries
  // no message, so the callback refetches, which is what settles anything
  // the snapshots missed (a failed turn's recorded run, most of all).
  //
  // Before this, the entry mapped `chat.done` alone. That event fires once,
  // at the very end of a turn, so nothing at all appeared while the agent
  // worked — no text, and no sign of the tool calls that are usually the
  // slowest part of it.
  //
  // The snapshot goes through the same translator a stored message does
  // (`lib/chat-message.ts`). It used to be passed through raw, so the answer
  // being written had no metadata at all: it was attributed to the chat's
  // title instead of the agent, carried no timestamp and no elapsed time, and
  // showed each of its tool calls twice and never as running.
  //
  // `chat.done` asks for a verbatim replace. At the end of a turn the stored
  // transcript is authoritative and the local one is not: a partial answer
  // from a turn that failed mid-stream, or the tail of a snapshot the merge
  // preferred because it had more parts, would otherwise stay on screen
  // forever beside the failure banner. The user's own message is persisted
  // before the turn is dispatched, so nothing of theirs is lost by replacing.
  "chat:refresh": {
    type: ["chat.message", "chat.done"],
    adapt: (event) =>
      event.type === "chat.message"
        ? { chatId: event.data?.["chat"], message: toLiveUiMessage(event.data?.["message"]) }
        : { chatId: event.data?.["chat"], replace: true },
  },

  // `layout/index.tsx`'s two `setProcessing(chatId, agentId, ...)` listeners,
  // which drive `ChatProcessingIndicator`'s "Atlas is working…".
  //
  // Both were `null` here, and the reasoning was sound at the time: no daemon
  // event said an agent had *started*, and neither `chat.delta` nor
  // `chat.done` carried an `agentId`, so the only way to fill one was to
  // fabricate it — which would have corrupted `AgentStore.occupancy` with a
  // phantom key. Rather than guess, the daemon now states both facts:
  // `chat.started` when a turn is picked up, and an `agent` field on
  // `chat.done` saying whose work ended.
  //
  // `chat.started` fires before the model has produced anything, which is the
  // point — on a reasoning model the wait for a first token is most of the
  // turn, and that is exactly the stretch the indicator exists to cover.
  "chat:start-processing": {
    type: "chat.started",
    adapt: (event) => ({ chatId: event.data?.["chat"], agentId: event.data?.["agent"] }),
  },
  "chat:end-processing": {
    type: "chat.done",
    adapt: (event) => ({ chatId: event.data?.["chat"], agentId: event.data?.["agent"] }),
  },

  // `changes-content.tsx`, `files/content/index.tsx`, `files-explorer-
  // group.tsx` — all read `payload.context` (which explorer pane) and
  // `payload.changes` (an array of `{path, ...}`).
  //
  // The daemon publishes `collection.changed` on every repository write
  // (`internal/app`'s `collectionPublisher`, wired for every repo in
  // `newRepoSet`) and on what the watcher sees on disk. An earlier version
  // of this comment said the event was "reserved, not live" and that
  // nothing published it; that was true when it was written and is not now,
  // and believing it is what kept the two readers below from being fixed.
  //
  // The payload's field names are `collection`/`key`/`op`/`path`
  // (`internal/core/collections.Changed`'s json tags). They were Go's own
  // capitalised names until those tags were added, so `data?.path` read
  // undefined and this adapter always produced an empty `changes` array —
  // an open file never refetched when it changed underneath.
  "files:changed": {
    type: "collection.changed",
    adapt: (event) => {
      const data = event.data as { path?: string; op?: string } | undefined;
      return {
        context: undefined,
        changes: data?.path ? [{ path: data.path, op: data.op }] : [],
      };
    },
  },

  // A record was written anywhere in the workspace — a project or goal an
  // agent created, an agent added from another window, a memory the
  // subconscious formed. The name is AOS's own rather than the original's,
  // because the original had no such event: it published a per-domain
  // `activity` and nothing else, so a change made outside the current
  // window reached the screen only if some *other* signal happened to force
  // a refetch.
  //
  // `lib/realtime.ts` already invalidates the react-query key for it. This
  // entry is what the workspace shell needs on top: the sidebar, the agent
  // pickers and the project/goal selectors read preloaded *stores*, which no
  // cache invalidation can reach.
  "records:changed": {
    type: "collection.changed",
    adapt: (event) => {
      const data = event.data as { collection?: string; op?: string; path?: string } | undefined;
      return { collection: data?.collection ?? "", op: data?.op ?? "", path: data?.path ?? "" };
    },
  },

  // `layout/index.tsx`: `notify(event)` then `router.invalidate()`. This is
  // the one real 1:1 match — the daemon's `activity` event (`internal/app/
  // continuity.go`'s `realtimeSink.OnActivity`) already carries a full
  // `activity.Activity` (`internal/domain/activity/entity.go`: `id,
  // namespace, event, title, body, icon, data, actor, actorType,
  // createdAt`), which lines up field-for-field with `use-notification.ts`'s
  // `NotificationPayload` — apart from `actions` (a client-side "quick
  // action buttons" annotation with no Go equivalent; an optional field,
  // its absence degrades the toast, not the data) and `workspaceId`, read
  // off the event envelope's own `Workspace` field rather than `Activity`
  // itself.
  // The approval channel's own event (`internal/app/runtime.go`'s
  // `approvalNotifier`), which fires twice per request: once when an agent
  // asks, and once when the request settles — the second carrying
  // `{id, settled: true, approved, reason}` instead of the request itself.
  // `ApprovalDialog` refetches on either, so a request answered from another
  // window (or expired) leaves this one's queue on its own.
  "approval:requested": "approval.request",

  "activity:created": {
    type: "activity",
    adapt: (event) => ({ ...(event.data as Record<string, unknown>), workspaceId: event.workspace }),
  },
};
