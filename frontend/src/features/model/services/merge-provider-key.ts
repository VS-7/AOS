import type { ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";

/**
 * Returns `providers` with `id`'s entry set to `key`, or removed entirely
 * when `key` is empty/whitespace — every other entry untouched.
 *
 * Kept in its own file, with no `@/app/aos` import, so a test can exercise
 * the one property that actually matters here — an edit to provider A
 * never touches provider B's key — without dragging in the whole `aos`
 * singleton's import graph (`model-provider.service.ts` pulls in the
 * router, every store, every feature) just to call a pure function.
 */
export function mergeProviderKey(
  providers: ConfigAgentProviderConnection[],
  id: string,
  key: string,
): ConfigAgentProviderConnection[] {
  const next = providers.filter((p) => p.id !== id);
  const trimmed = key.trim();
  if (trimmed) {
    next.push({ id, key: trimmed });
  }
  return next;
}
