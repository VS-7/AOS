/**
 * Shared cold-start state for every desktop transport (client.ts's
 * DomainService.Invoke, auth.ts's AuthService calls, ...).
 *
 * A rejected Call.ByName looks identical whether there's truly no Wails host
 * (a plain browser tab) or there is one that just hasn't warmed up yet.
 * Patient retries are right for the second case and expensive for the
 * first — a browser tab would eat several seconds on every call, forever,
 * if it always retried. confirmedDesktop breaks the tie: any successful
 * desktop call, from *any* service, proves the host is real, after which
 * every other service's failures are assumed transient too. Before that
 * first success, patience is bounded to the window right after the page
 * loads, when a real host warming up is the likely explanation; past it,
 * one quick attempt is enough to conclude this is not the desktop.
 *
 * This used to be private to client.ts, duplicated (with a shorter, un-
 * synchronised budget) in auth.ts. Splitting it out means a page's first
 * successful call — auth's Login, say — immediately stops every other
 * service on the page from paying its own separate cold-start tax.
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
