import { daemonOrigin } from "@/lib/daemon-origin";
import { getWorkspace } from "@/lib/client";

/**
 * Where a routine's webhook is fired, and how.
 *
 * The daemon's route is `POST /api/hooks/routines/{id}` with the routine's
 * token as a bearer credential (internal/transport/httpapi, RoutineWebhookPath).
 * The workspace rides in the query string because the sender carries no
 * header of this system's; it is the one this window addresses.
 */
export class RoutineWebhookHelper {
  /** The address to POST to. Loopback unless a tunnel exposes the daemon. */
  public static fireUrl(routineId: string, origin: string = RoutineWebhookHelper.origin()): string {
    const workspace = getWorkspace();
    const query = workspace ? `?workspace=${encodeURIComponent(workspace)}` : "";
    return `${origin}/api/hooks/routines/${encodeURIComponent(routineId)}${query}`;
  }

  /** A command that fires it, for a person to paste into a terminal or a CI step. */
  public static curlExample(fireUrl: string, token: string): string {
    return `curl -X POST '${fireUrl}' \\\n  -H 'Authorization: Bearer ${token}' \\\n  -H 'Content-Type: application/json' \\\n  -d '{}'`;
  }

  /** The daemon's origin: the window states it; a browser tab is served by it. */
  public static origin(): string {
    if (daemonOrigin) return daemonOrigin;
    return typeof window !== "undefined" ? window.location.origin : "";
  }
}

/**
 * A token handed out by a create that is about to navigate away.
 *
 * Creating a routine moves the page from /routines/new to /routines/<id>,
 * and the token Go returns is shown once, ever — so it is held here across
 * that navigation and taken by the page it lands on.
 */
const pending = new Map<string, string>();

export const pendingWebhookTokens = {
  hold(routineId: string, token: string) {
    pending.set(routineId, token);
  },
  take(routineId: string): string | undefined {
    const token = pending.get(routineId);
    pending.delete(routineId);
    return token;
  },
};
