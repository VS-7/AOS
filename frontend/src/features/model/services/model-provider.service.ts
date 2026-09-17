import * as React from "react";
import { aos } from "@/app/aos";
import { PROVIDER_CATALOG } from "@/features/model/data/provider-catalog";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import type { Config, ConfigAgentModels, ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";
import { clearSlotsOf, mergeProviderKey, removeProvider, seedDefaultSlot } from "./merge-provider-key";
import { useDiscoveredModels } from "./discovered-models";

export { mergeProviderKey, removeProvider };
export { MODEL_DISCOVERY_KEY } from "./discovered-models";

/**
 * The seam between three sources, none of which knows about the others.
 *
 * - `provider-catalog.ts` — what a provider *is*: its name, its description,
 *   how it authenticates. Nothing publishes this, so it is static.
 * - `Config.agents.providers`/`.models` (`internal/domain/config/entity.go`)
 *   — which providers are connected and which model each slot uses. Reached
 *   through the real `config.get`/`config.update`; `model.set` stays dormant
 *   because connecting a provider is a configuration write here, not a
 *   command of its own.
 * - `models_list` (`internal/domain/model/commands.go`) — what a connected
 *   provider actually serves, asked of the provider itself.
 *
 * The connected/disconnected half needs no network call at all: it is read
 * from the `Config` already in route context. The catalogue half does, and
 * is cached on both sides — see `discovered-models.ts`.
 */

/**
 * Every known provider: what it is from the catalog, whether it is connected
 * from the live config, and what it serves from the provider itself.
 *
 * The models are the part that used to be a lie. A connected provider's list
 * now comes from `models_list`, which asks that provider's own catalogue
 * endpoint with this installation's credential. The static list survives as
 * the fallback for the two cases discovery cannot answer: a provider that is
 * not connected yet (nothing to authenticate with) and one that failed to
 * answer (where showing the last known good names beats showing none).
 */
export function useModelProviders(): ModelProvider[] {
  const context = aos.useContext();
  const providers = (context.config?.agents?.providers ??
    []) as ConfigAgentProviderConnection[];

  // Presence of the entry is what "connected" means, not the key being
  // non-empty. An `oauth-file` provider (`codex`, `gemini-cli`) is connected
  // with no key at all — its credential is another tool's file on this
  // machine. Requiring a non-empty key here meant those two could never show
  // as connected even once their entry was saved, so they stayed in the
  // "Connect" menu forever. An `api-key` provider can't reach this state: its
  // dialog refuses to submit a blank key.
  const connected = React.useMemo(
    () => new Set(providers.map((p) => p.id).filter(Boolean)),
    [providers],
  );

  // Nothing connected means nothing to ask, and asking anyway would be a
  // round trip to learn that.
  const discovery = useDiscoveredModels(connected.size > 0);

  return React.useMemo(
    () =>
      PROVIDER_CATALOG.map((entry) => {
        const found = discovery.models.get(entry.id);
        const discovered = !!found && found.length > 0;
        return {
          ...entry,
          configured: connected.has(entry.id),
          models: discovered ? found : entry.models,
          modelsDiscovered: discovered,
          modelsError: discovery.errors.get(entry.id),
          modelsErrorActions: discovery.actions.get(entry.id),
        };
      }),
    [connected, discovery],
  );
}

/** What connecting a provider found out when it asked the provider itself. */
export interface ProviderConnectOutcome {
  /** The provider's own reason, when it could not list its models. */
  error?: string;
  /** What to do about `error`, most specific first. */
  actions?: string[];
  /** The model the Default slot was pointed at, when this connect seeded it. */
  seeded?: string;
}

/**
 * Connects a provider, or updates the credential of one already connected.
 *
 * An empty `key` is a legitimate connection, not a request to disconnect —
 * see `mergeProviderKey`. Use `disconnectModelProvider` to remove one.
 *
 * Reads with `reveal: true` first — `config.get`'s default view redacts
 * every provider key to a fingerprint (`internal/domain/config/redact.go`,
 * ADR-0010) — before writing the whole `agents.providers` array back,
 * since `patch.Apply` treats it as one leaf: a caller replaces the whole
 * list, there is no path to one element inside it. Round-tripping the
 * redacted view instead would silently overwrite every untouched
 * provider's real key with its fingerprint string on the very next save.
 *
 * Resolves with what the provider said. The dialog used to report
 * "connected." for a key the provider had just refused, and Default was then
 * seeded from the static list — a screen that looked ready over a first chat
 * that could only fail.
 */
export async function setModelProviderKey(
  id: string,
  key: string,
): Promise<ProviderConnectOutcome> {
  // Two writes, in this order, because the second depends on the first: the
  // provider can only be asked what it serves once this installation holds a
  // credential to ask with. Seeding from the static catalog in one write is
  // what this used to do, and it is how `gpt-5.3-codex-spark` — a model that
  // does not exist — became the saved default for anyone connecting Codex.
  await writeProviders((providers) => mergeProviderKey(providers, id, key));

  const discovery = await firstModelOf(id);
  if (discovery.error || !discovery.model) {
    return { error: discovery.error, actions: discovery.actions };
  }
  const model = discovery.model;
  let seeded: string | undefined;
  await writeProviders(
    (providers) => providers,
    (models, providers) => {
      const next = seedDefaultSlot(models, providers, id, model);
      if (next !== models) seeded = model;
      return next;
    },
  );
  return { seeded };
}

/**
 * The model to offer as this provider's default: the first one it lists.
 *
 * First, not chosen: where a provider publishes a ranking the adapter
 * preserves it (`internal/runtime/providers/openai/models.go` keeps the
 * Codex endpoint's own priority order), so the first entry is the provider's
 * recommendation rather than this build's guess.
 *
 * A provider that *answered* with a failure — a refused key, a login file
 * that is not there — seeds nothing and says why: the static list is what
 * was true when somebody typed it, and pointing Default at it is how a
 * refused key looked configured. Only a round trip that never reached the
 * daemon falls back to the static list, so a hiccup there does not leave a
 * working provider with an empty slot.
 */
async function firstModelOf(
  id: string,
): Promise<{ model?: string; error?: string; actions?: string[] }> {
  const fallback = PROVIDER_CATALOG.find((p) => p.id === id)?.models[0]?.id;
  try {
    const answer = await aos.client.model.list.query<{
      providers?: { id?: string; models?: { id?: string }[]; error?: string; cta?: { label?: string }[] }[];
    }>({ query: { provider: id } });

    const found = answer.data?.providers?.find((p) => p?.id === id);
    if (found?.error) {
      const actions = (found.cta ?? [])
        .map((cta) => cta?.label?.trim())
        .filter((label): label is string => !!label);
      return { error: found.error, actions };
    }
    const first = found?.models?.find((m) => m?.id)?.id;
    if (first) return { model: first };
  } catch {
    // Discovery is an improvement on the fallback, never a precondition for
    // connecting. A round trip that failed says nothing about the provider.
  }
  return { model: fallback };
}

/**
 * Disconnects a provider — the meaning an empty key used to carry.
 *
 * Every model slot pointing at it is cleared in the same write. Left behind,
 * the Default slot kept naming a provider with no credential: the screen
 * showed it as unset, connecting another provider did not fill it (the slot
 * looked owned), and the next chat went out to the disconnected provider
 * with no key and came back as its 401.
 */
export async function disconnectModelProvider(id: string): Promise<void> {
  await writeProviders(
    (providers) => removeProvider(providers, id),
    (models) => clearSlotsOf(models, id),
  );
}

/**
 * Reads the *revealed* provider list, applies `edit`, and writes the whole
 * array back.
 *
 * `reveal: true` matters: `config.get`'s default view redacts every
 * provider key to a fingerprint (`internal/domain/config/redact.go`,
 * ADR-0010), and `patch.Apply` treats `agents.providers` as one leaf — a
 * caller replaces the whole list, there is no path to one element inside
 * it. Round-tripping the redacted view would therefore overwrite every
 * untouched provider's real key with its fingerprint string on the very
 * next save.
 */
async function writeProviders(
  edit: (providers: ConfigAgentProviderConnection[]) => ConfigAgentProviderConnection[],
  editModels?: (
    models: ConfigAgentModels | undefined,
    providers: ConfigAgentProviderConnection[],
  ) => ConfigAgentModels | undefined,
): Promise<void> {
  const current = await aos.client.config.get.query({
    query: { reveal: true },
  });
  if (current.error) {
    throw new Error(
      current.error.message ?? "Unable to read the current configuration.",
    );
  }

  const config = current.data as Config | undefined;
  const providers = edit(config?.agents?.providers ?? []);
  const models = editModels?.(config?.agents?.models, providers);

  const result = await aos.client.config.update.mutate({
    body: {
      agents: models === undefined ? { providers } : { providers, models },
    },
  });
  if (result.error) {
    throw new Error(
      (result.error as { message?: string })?.message ??
        "Unable to save the provider.",
    );
  }

  // The redacted, shared config (route context, every other settings
  // section) only reflects this write once the store is told to refetch.
  await aos.stores.config.actions.refresh();
}
