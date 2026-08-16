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
        dispatch(queryClient, event);
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
    case "chat.done":
      void queryClient.invalidateQueries({ queryKey: ["chat", event.data?.["chat"]] });
      break;
    case "activity":
      void queryClient.invalidateQueries({ queryKey: ["activity"] });
      // An activity is published by whatever mutation caused it, so it is also
      // the signal that the thing it is about changed.
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
