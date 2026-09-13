/**
 * Shared state for every desktop transport (client.ts's DomainService.Invoke,
 * auth.ts's AuthService calls, ...).
 *
 * confirmedDesktop records that a bridge call has been answered, from *any*
 * service — which is what `isDesktop()` still reports.
 *
 * The backoff below is for one kind of answer only: one that proves the call
 * never reached the daemon (AOS_DAEMON_UNREACHABLE from a daemon the window
 * started a moment ago), which callBridge lets a caller wait out. It used to
 * be spent on any rejection that carried no answer, as a bridge "still warming
 * up" — and a rejection with no answer is exactly the one that cannot say
 * whether the call ran, so a command could be sent twice. The backoff is
 * offered in the first five seconds after the page loads, and on a page whose
 * bridge has answered before.
 */

const RETRY_DELAYS_MS = [200, 500, 1000, 2000];

let confirmedDesktop = false;
const desktopColdStartUntil = typeof window === "undefined" ? 0 : Date.now() + 5_000;

/** Call once a desktop call actually succeeds. */
export function markDesktopConfirmed(): void {
  confirmedDesktop = true;
}

/** Whether this page has ever completed a real desktop call. */
export function isDesktopConfirmed(): boolean {
  return confirmedDesktop;
}

/** The backoff schedule to retry a failed desktop call with, right now. */
export function desktopRetryDelays(): readonly number[] {
  if (confirmedDesktop || Date.now() < desktopColdStartUntil) return RETRY_DELAYS_MS;
  return [];
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
