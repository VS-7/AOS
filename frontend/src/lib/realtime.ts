import { useEffect, useRef, useState } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { getWorkspace } from "./client";

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
 * the ported code's translation point for Fractal's event names) listens
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

    const connect = () => {
      if (closed) return;
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const workspace = getWorkspace();
      const url = `${protocol}//${window.location.host}/ws${workspace ? `?workspace=${encodeURIComponent(workspace)}` : ""}`;

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
    case "approval.request":
      void queryClient.invalidateQueries({ queryKey: ["approvals"] });
      break;
    default:
      break;
  }
}

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
