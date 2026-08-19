/**
 * Minimal local declaration, same spirit as `core/interfaces/response.
 * interfaces.ts`.
 *
 * The real source (`_extracted/v401/server/src/@core/services/activity.
 * ts`) builds a server-side notification registry (`FractalActivity.
 * create(...)`) that merges every one of the 26 features' own
 * `notifications/*.notification.ts` files through the `@igniter-js/core`-
 * coupled `FractalNotification` builder — none of which this port copied
 * in (same reasoning as `errors/`).
 *
 * `features/config/interfaces/config.interfaces.ts` (this file's only
 * consumer) only projects `FractalActivityInstance["events"]["config"]`
 * to type one field — a loose index signature is enough for that
 * projection to resolve.
 */
export interface FractalActivityInstance {
  events: Record<string, any>;
}
