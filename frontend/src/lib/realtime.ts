import { useEffect, useRef, useState } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { getWorkspace, system } from "./client";

/** One event from the daemon's realtime channel. */
export interface RealtimeEvent {
  type: string;
  workspace?: string;
  data?: Record<string, unknown>;
}

/** How the connection is doing, for the indicator in the corner. */
export type ConnectionState = "connecting" | "open" | "reconnecting" | "closed";

/**
 * Raw-event subscribers, outside the query-cache dispatch below.
 *
 * `useRealtime(queryClient)` owns the one WebSocket the app opens — every
 * other piece of the app that wants a live event (`hooks/use-realtime.ts`,
 * the ported code's translation point for AOS's event names) listens
 * here instead of opening a socket of its own. Two sockets was the B1
 * defect this exists to prevent: one spoke the daemon's real event names
 * and carried the workspace-scoping fix, the other didn't and lacked it.
 */
const rawListeners = new Set<(event: RealtimeEvent) => void>();

/**
 * Subscribes to every event this channel receives, unfiltered — the
 * subscriber decides what `event.type` it cares about.
 *
 * This is the one seam other code should use to react to realtime traffic.
 * Reaching for `new WebSocket(...)` anywhere else in the app recreates the
 * exact defect this function exists to close off.
 */
export function onRealtimeEvent(listener: (event: RealtimeEvent) => void): () => void {
  rawListeners.add(listener);
  return () => {
    rawListeners.delete(listener);
  };
}

/**
 * Backoff between reconnection attempts.
 *
 * It grows and it stops growing. An unbounded backoff means a laptop that was
 * asleep for an hour comes back and waits another hour before noticing the
 * daemon is up; a fixed short one means a daemon that is down gets hammered by
 * every open window.
 */
const BACKOFF_MS = [250, 500, 1_000, 2_000, 5_000, 10_000] as const;

function backoffFor(attempt: number): number {
  return BACKOFF_MS[Math.min(attempt, BACKOFF_MS.length - 1)] ?? 10_000;
}

/**
 * The origin to open the event channel against.
 *
 * Same-origin is right in a browser, where the daemon serves the bundle
 * itself. It is wrong in the desktop window, where the page comes from the
 * application binary's own embedded assets: `window.location.host` there is
 * the asset host, which serves no `/ws`, so the socket connected to nothing
 * and every live update in the application silently never arrived. The
 * daemon's real address is known to the Go side and asked for here.
 *
 * Resolved once and remembered — it cannot change while the window is open,
 * and a reconnect should not pay for the bridge call again.
 */
let daemonOrigin: string | null = null;

/**
 * Read at module load, before anything can navigate.
 *
 * The window states the daemon's address in its own URL, and the router
 * rewrites that URL on the first navigation — which happens well before the
 * socket is ready to open, since it waits for a workspace first. Reading it
 * lazily meant reading it after it was already gone.
 */
const declaredDaemon =
  typeof window === "undefined"
    ? null
    : new URLSearchParams(window.location.search).get("daemon");

async function originForSocket(): Promise<string> {
  if (daemonOrigin !== null) return daemonOrigin;

  // What the window declared (`cmd/aos-desktop`'s
  // WebviewWindowOptions.URL) — see declaredDaemon above.
  if (declaredDaemon) {
    daemonOrigin = declaredDaemon.replace(/\/+$/, "");
    return daemonOrigin;
  }

  // Failing that, ask the bridge.
  try {
    const address = await system.daemonAddress();
    if (address) {
      daemonOrigin = address.replace(/\/+$/, "");
      return daemonOrigin;
    }
  } catch {
    // No Wails host — a browser tab, where the page's own origin is the
    // daemon and the answer below is right.
  }

  daemonOrigin = window.location.origin;
  return daemonOrigin;
}

/**
 * Keeps one connection to the workspace channel and pushes what arrives into
 * the query cache.
 *
 * Dispatching into the cache rather than into component state is what keeps the
 * subscription bookkeeping out of the components: a screen asks for the data it
 * needs and re-renders when that data changes, whether the change came from its
 * own mutation or from an agent working in the background.
 */
export function useRealtime(queryClient: QueryClient): ConnectionState {
  const [state, setState] = useState<ConnectionState>("connecting");
  const socket = useRef<WebSocket | null>(null);

  useEffect(() => {
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let closed = false;

    // Inside the desktop window the channel arrives over the Wails bridge,
    // relayed by the application process (`cmd/aos-desktop`'s
    // forwardRealtime). A WebView will not let a page served from the
    // application's own `wails://` scheme open a ws:// socket to the daemon
    // — the browser refuses it locally, so nothing reaches the daemon to
    // even be refused there. Every live update in the desktop was lost to
    // that, silently.
    if (declaredDaemon) {
      const off = Events.On("aos:realtime", (event: { data?: unknown }) => {
        const payload = event?.data as RealtimeEvent | RealtimeEvent[] | undefined;
        // Wails delivers a single emitted value as a one-element array in
        // some versions and bare in others; accept both rather than depend
        // on which.
        const one = Array.isArray(payload) ? payload[0] : payload;
        if (one && typeof one === "object" && "type" in one) {
          deliver(queryClient, one as RealtimeEvent);
        }
      });
      setState("open");
      return () => {
        closed = true;
        setState("closed");
        off();
      };
    }

    const connect = () => {
      if (closed) return;
      const workspace = getWorkspace();

      // The daemon refuses a socket that names no workspace — it has no
      // channel to subscribe it to (`realtime.Upgrade`: "a workspace must be
      // named"). At first paint none is known yet: the workspace store
      // resolves it a moment later and publishes it through `setWorkspace`.
      // Connecting anyway burns a real attempt on a guaranteed 400 and
      // advances the backoff, so by the time the id exists the retry is
      // seconds away. Waiting costs one short poll and connects on the first
      // try instead.
      if (!workspace) {
        timer = setTimeout(connect, 250);
        return;
      }

      void openAt(workspace);
    };

    const openAt = async (workspace: string) => {
      const origin = await originForSocket();
      if (closed) return;
      if (!/^https?:/.test(origin)) {
        // Nothing named a reachable daemon, and this page's own origin is
        // not one either (the desktop's `wails://` scheme). Opening a
        // socket at it throws; saying so beats a silent dead channel.
        console.error(
          `[realtime] no daemon address to open the event channel against (origin ${origin}) — live updates are off`,
        );
        setState("closed");
        return;
      }
      const url = `${origin.replace(/^http/, "ws")}/ws?workspace=${encodeURIComponent(workspace)}`;

      const ws = new WebSocket(url);
      socket.current = ws;

      ws.onopen = () => {
        attempt = 0;
        setState("open");
      };
      ws.onmessage = (message) => {
        let event: RealtimeEvent;
        try {
          event = JSON.parse(message.data as string) as RealtimeEvent;
        } catch {
          // A frame that is not JSON is a frame from something that is not the
          // daemon. Dropping it beats crashing the page.
          return;
        }
        deliver(queryClient, event);
      };
      ws.onclose = () => {
        if (closed) return;
        setState("reconnecting");
        timer = setTimeout(connect, backoffFor(attempt++));
      };
      ws.onerror = () => ws.close();
    };

    connect();
    return () => {
      closed = true;
      setState("closed");
      if (timer) clearTimeout(timer);
      socket.current?.close();
    };
  }, [queryClient]);

  return state;
}

/**
 * What happens to one event once it arrives: the cache invalidations it
 * implies, and fan-out to whoever else is listening raw (`onRealtimeEvent`
 * subscribers — see that function's own comment). This is the single
 * function `ws.onmessage` calls; tests exercise this directly instead of
 * standing up a real WebSocket.
 */
export function deliver(queryClient: QueryClient, event: RealtimeEvent): void {
  dispatch(queryClient, event);
  for (const listener of rawListeners) listener(event);
}

/**
 * Turns one event into the cache invalidations it implies.
 *
 * The mapping is deliberately coarse: a task moved invalidates the tasks, not
 * the one task. A finer mapping saves a request and costs a class of bug where
 * a screen shows one record that is fresh next to a list that is stale.
 */
function dispatch(queryClient: QueryClient, event: RealtimeEvent): void {
  switch (event.type) {
    case "chat.delta":
      appendDelta(queryClient, event);
      break;
    case "chat.done": {
      // B1(c) of the final review: this used to invalidate `["chat",
      // <chatId>]` — a key from a hand-built screen Task 10 retired. The
      // facade's `useQuery` computes its key as `[feature, action,
      // flattenArgs(opts)]` (`aos-facade.ts`'s `actionNode`), so a live
      // chat read sits at `["chat", "getById", { chat: <chatId> }]`; the
      // old two-element key could never match that shape, silently.
      // `predicate` (rather than a bare `queryKey` prefix) keeps this
      // scoped to the one chat that actually finished a turn — matching
      // this function's own "coarse invalidation" philosophy above would
      // mean a completed turn in one open chat tab refetches every other
      // open tab's `getById` too, which is the "screen shows one record
      // that is fresh next to a list that is stale" failure mode in
      // reverse (freshness by way of over-fetching), not the class of bug
      // that doc comment is actually accepting the cost of.
      const chatId = event.data?.["chat"];
      void queryClient.invalidateQueries({
        predicate: (q) =>
          q.queryKey[0] === "chat" &&
          q.queryKey[1] === "getById" &&
          (typeof chatId !== "string" ||
            (q.queryKey[2] as Record<string, unknown> | undefined)?.["chat"] === chatId),
      });
      break;
    }
    case "activity":
      // `["activity"]` is already a valid prefix of the facade's
      // `["activity", "list", {...}]` key — react-query's default
      // `exact: false` matching walks the target key element-by-element
      // against this one, so a one-element key here was never the bug;
      // left as-is.
      void queryClient.invalidateQueries({ queryKey: ["activity"] });
      // An activity is published by whatever mutation caused it, so it is also
      // the signal that the thing it is about changed. Same reasoning: a
      // single-element `[namespace]` key is already a valid facade-shaped
      // prefix (`[feature, action, args]` with the last two omitted), so
      // this one needed no migration either.
      if (typeof event.data?.["namespace"] === "string") {
        void queryClient.invalidateQueries({ queryKey: [event.data["namespace"] as string] });
      }
      break;
    case "collection.changed": {
      // Every repository write publishes this — a project or goal an agent
      // created, an agent added from another window, a memory the
      // subconscious formed. It used to fall through to `default: break`, so
      // the daemon said "this changed" and the interface did nothing with
      // it, which is most of "the screen does not update by itself".
      const collection = event.data?.["collection"];
      if (typeof collection !== "string" || collection === "") break;
      const feature = FEATURE_OF_COLLECTION[collection];
      if (feature) {
        void queryClient.invalidateQueries({ queryKey: [feature] });
        break;
      }
      // A collection nobody declared a feature for is a *dynamic* one — a
      // collection an agent or a skill created. Those are read through the
      // `collection` feature's own record actions, so the invalidation goes
      // there rather than being dropped.
      void queryClient.invalidateQueries({ queryKey: ["collection"] });
      break;
    }
    case "approval.request":
      // `["approval"]`, not `["approvals"]`: the facade keys a query by its
      // feature name (`aos-facade.ts`'s `actionNode`), and the feature is
      // `approval` — `approval.list` in `command-map.ts`. The plural was a
      // leftover from a hand-built dialog that no longer exists, and it
      // matched nothing.
      void queryClient.invalidateQueries({ queryKey: ["approval"] });
      break;
    default:
      break;
  }
}

/**
 * Which facade feature a collection's writes belong to.
 *
 * The daemon names collections in the plural, because that is the directory
 * under `.aos/`; the interface keys its cache by the singular feature name in
 * `command-map.ts` (`project.list`, `goal.get`, …). This is the one place
 * that spelling difference is resolved — the same role `command-map.ts` plays
 * for calls and `realtime-event-map.ts` for event names.
 *
 * A collection missing from this table is not an error: it is a dynamic
 * collection, handled by the `default` above.
 */
const FEATURE_OF_COLLECTION: Record<string, string> = {
  agents: "agent",
  artifacts: "artifact",
  chats: "chat",
  collections: "collection",
  comments: "comment",
  goals: "goal",
  instructions: "instruction",
  memories: "memory",
  projects: "project",
  routines: "routine",
  runs: "routine",
  skills: "skill",
  tasks: "task",
  templates: "template",
  todos: "todo",
  toolsets: "toolset",
  views: "view",
};

// Disclosed gap, not fixed here: nothing in the tree currently reads
// `["chat", <id>, "streaming"]`. The ported chat UI (`use-chat.ts`) only
// reconciles on `chat:refresh` (translated to `chat.done` — see
// `hooks/use-realtime.ts`'s `REALTIME_EVENT_MAP`), which fires once per
// turn, not per token; live token-by-token display would mean building
// message snapshots out of accumulated `chat.delta` text, which is a chat
// UI feature, not a realtime-transport fix. Left wired (not removed) since
// it is exactly the write a future consumer would read.
/** One streamed piece of an answer, as the chat screen accumulates it. */
export interface StreamingAnswer {
  text: string;
  reasoning: string;
}

/**
 * Accumulates a delta into the cache for the conversation it belongs to.
 *
 * The conversation itself is not refetched per token: the deltas build a
 * separate value the composer reads, and the stored transcript replaces it when
 * the turn ends. Refetching per token would be a request per word.
 */
function appendDelta(queryClient: QueryClient, event: RealtimeEvent): void {
  const chat = event.data?.["chat"];
  if (typeof chat !== "string") return;

  queryClient.setQueryData<StreamingAnswer>(["chat", chat, "streaming"], (current) => ({
    text: (current?.text ?? "") + String(event.data?.["text"] ?? ""),
    reasoning: (current?.reasoning ?? "") + String(event.data?.["reasoning"] ?? ""),
  }));
}
