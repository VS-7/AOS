import type { ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";

/**
 * Returns `providers` with `id`'s entry set to `key` — every other entry
 * untouched.
 *
 * An empty `key` still writes an entry, and that is the whole point.
 * `oauth-file` providers (`codex`, `gemini-cli`) are connected precisely
 * by having no key of their own: the credential belongs to another tool
 * on this machine, and their own dialog says so ("nothing to enter here
 * unless you want to override it"). Go agrees — `providers.Config.APIKey`
 * (`internal/runtime/providers/registry.go`) documents empty as *normal*
 * for exactly these.
 *
 * This function used to delete the entry whenever the key was blank, so
 * connecting one of those two saved `agents.providers: []` — a write that
 * returned 200, showed a "connected" toast, and left nothing behind.
 * Emptiness cannot carry the "forget this provider" meaning as well;
 * `removeProvider` below says it explicitly instead.
 *
 * Kept in its own file, with no `@/app/aos` import, so a test can
 * exercise the one property that actually matters here — an edit to
 * provider A never touches provider B's key — without dragging in the
 * whole `aos` singleton's import graph (`model-provider.service.ts` pulls
 * in the router, every store, every feature) just to call a pure
 * function.
 */
export function mergeProviderKey(
  providers: ConfigAgentProviderConnection[],
  id: string,
  key: string,
): ConfigAgentProviderConnection[] {
  const next = providers.filter((p) => p.id !== id);
  next.push({ id, key: key.trim() });
  return next;
}

/**
 * Returns `providers` without `id`'s entry — the disconnect side, which
 * `mergeProviderKey`'s blank key used to stand in for.
 */
export function removeProvider(
  providers: ConfigAgentProviderConnection[],
  id: string,
): ConfigAgentProviderConnection[] {
  return providers.filter((p) => p.id !== id);
}
