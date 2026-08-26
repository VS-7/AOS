import { useCallback, useEffect, useRef } from "react";
import { useRouter } from "@tanstack/react-router";

/**
 * How long to wait for more events before refetching.
 *
 * Long enough that a burst becomes one refetch, short enough that a single
 * event still feels immediate. An agent working produces events in clusters —
 * a tool call, its result, a status change, an activity entry, all within a
 * few dozen milliseconds — and this is sized to cover a cluster.
 */
const COALESCE_MS = 250;

/**
 * A ceiling on how long coalescing can defer a refetch.
 *
 * Without it, a stream of events arriving faster than COALESCE_MS would push
 * the deadline forever and the screen would never update while an agent was
 * busy — which is exactly when it most needs to.
 */
const MAX_DEFER_MS = 1_000;

/**
 * `router.invalidate()`, coalesced.
 *
 * Invalidating re-runs every loader on the current route, which for this
 * application means a round of commands per screen — the task list, its
 * counts, the agent directory, the activity feed. That is the right response
 * to something having changed, and the wrong response to something having
 * changed forty times in a second.
 *
 * The realtime channel does exactly that. An agent taking a turn emits
 * activity events continuously, and the workspace shell called
 * `await router.invalidate()` from the handler for each one. Every event
 * therefore triggered a full refetch of the visible screen, each refetch
 * re-rendered the tree, and the re-renders queued behind more events: the
 * window stopped responding for as long as the agent kept working, which is
 * the freeze this exists to remove.
 *
 * Coalescing keeps the behaviour — the screen still reconciles against the
 * server after a change — while paying for it once per burst instead of once
 * per event.
 */
export function useCoalescedInvalidate(): () => void {
  const router = useRouter();

  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // When the currently-pending burst started, so a continuous stream still
  // refetches every MAX_DEFER_MS rather than being pushed back forever.
  const firstRequest = useRef(0);

  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );

  return useCallback(() => {
    const now = performance.now();

    if (timer.current === null) {
      firstRequest.current = now;
    } else {
      if (now - firstRequest.current >= MAX_DEFER_MS) {
        // The burst has gone on long enough; let the pending refetch run on
        // schedule rather than deferring it again.
        return;
      }
      clearTimeout(timer.current);
    }

    timer.current = setTimeout(() => {
      timer.current = null;
      void router.invalidate();
    }, COALESCE_MS);
  }, [router]);
}
