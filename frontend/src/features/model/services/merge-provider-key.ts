import type { ConfigAgentModels, ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";

/**
 * Returns `providers` with `id`'s entry set to `key` — every other entry
 * untouched.
 *
 * An empty `key` still writes an entry, and that is the whole point.
 * `oauth-file` providers (`codex`, `gemini-cli`, `antigravity`) are
 * connected precisely by having no key of their own: the credential belongs
 * to another tool on this machine, and their dialog has nothing to type
 * into. Go agrees — `providers.Config.APIKey`
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

/**
 * Points the `default` model slot at `providerId` when nothing usable owns it.
 *
 * Connecting a provider and being able to talk to an agent are the same
 * intent, but they were two different pieces of configuration, and only
 * one of them had a control that wrote anything. `agents.models.default`
 * is the single slot the runtime reads to answer a chat
 * (`internal/app/runtime.go`'s `models.For`; `subconscious` falls back to
 * it in `continuity.go`), and with it unset `agentloop.Resolve` returns
 * `AOS_AGENT_PROVIDER_NOT_ENABLED`. So a person could connect a provider,
 * watch the Models list show a model next to "Default", send a message,
 * and get silence.
 *
 * Seeding here rather than at render time keeps the write tied to
 * something the person actually did, and it only ever fills a slot nobody
 * can use — a deliberate choice of a connected provider is never
 * overwritten by connecting another one. A slot naming a provider that is
 * not in `providers` (the list after this edit) counts as empty: the Models
 * screen already shows it as unset, and treating it as owned is what left
 * Default pointing at a disconnected provider after connecting a new one.
 *
 * `modelId` is passed in rather than looked up because the caller is the
 * one that can ask the provider (see `firstModelOf`). That is what covers
 * `openrouter`/`crof`/`opencode`, whose static catalogues are empty on
 * purpose.
 *
 * Returns `models` itself, unchanged, when there is nothing to seed.
 */
export function seedDefaultSlot(
  models: ConfigAgentModels | undefined,
  providers: ConfigAgentProviderConnection[],
  providerId: string,
  modelId: string,
): ConfigAgentModels | undefined {
  const current = models?.default;
  const owned =
    !!current?.provider &&
    !!current?.model &&
    providers.some((p) => p.id === current.provider);
  if (owned) return models;

  return {
    ...(models ?? {}),
    default: {
      provider: providerId,
      model: modelId,
      reasoning: current?.reasoning ?? "medium",
    },
  } as ConfigAgentModels;
}

/**
 * Returns `models` without the slots that name `providerId` — the model
 * side of a disconnect. Returns `models` itself when none did.
 */
export function clearSlotsOf(
  models: ConfigAgentModels | undefined,
  providerId: string,
): ConfigAgentModels | undefined {
  if (!models) return models;
  const entries = Object.entries(models) as [string, { provider?: string } | undefined][];
  if (!entries.some(([, slot]) => slot?.provider === providerId)) return models;
  return Object.fromEntries(
    entries.filter(([, slot]) => slot?.provider !== providerId),
  ) as ConfigAgentModels;
}
