import * as React from "react";
import { aos } from "@/app/aos";
import { PROVIDER_CATALOG } from "@/features/model/data/provider-catalog";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import type { Config, ConfigAgentModels, ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";
import { mergeProviderKey, removeProvider } from "./merge-provider-key";
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
        };
      }),
    [connected, discovery],
  );
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
 */
export async function setModelProviderKey(
  id: string,
  key: string,
): Promise<void> {
  // Two writes, in this order, because the second depends on the first: the
  // provider can only be asked what it serves once this installation holds a
  // credential to ask with. Seeding from the static catalog in one write is
  // what this used to do, and it is how `gpt-5.3-codex-spark` — a model that
  // does not exist — became the saved default for anyone connecting Codex.
  await writeProviders((providers) => mergeProviderKey(providers, id, key));

  const model = await firstModelOf(id);
  if (!model) return;
  await writeProviders(
    (providers) => providers,
    (models) => seedDefaultSlot(models, id, model),
  );
}

/**
 * The model to offer as this provider's default: the first one it lists.
 *
 * First, not chosen: where a provider publishes a ranking the adapter
 * preserves it (`internal/runtime/providers/openai/models.go` keeps the
 * Codex endpoint's own priority order), so the first entry is the provider's
 * recommendation rather than this build's guess.
 *
 * A provider that cannot be reached falls back to the static catalog, which
 * is the same list this function replaces — no worse than before, and it
 * keeps a network hiccup during connect from leaving the slot empty and the
 * agent unable to answer.
 */
async function firstModelOf(id: string): Promise<string | undefined> {
  try {
    const answer = await aos.client.model.list.query<{
      providers?: { id?: string; models?: { id?: string }[] }[];
    }>({ query: { provider: id } });

    const discovered = answer.data?.providers?.find((p) => p?.id === id)?.models;
    const first = discovered?.find((m) => m?.id)?.id;
    if (first) return first;
  } catch {
    // Discovery is an improvement on the fallback, never a precondition for
    // connecting. A provider that refuses to list its models can still serve
    // them, so a failure here must not fail the connection.
  }
  return PROVIDER_CATALOG.find((p) => p.id === id)?.models[0]?.id;
}

/**
 * Points the `default` model slot at `providerId` when nothing owns it yet.
 *
 * Connecting a provider and being able to talk to an agent are the same
 * intent, but they were two different pieces of configuration, and only
 * one of them had a control that wrote anything. `agents.models.default`
 * is the single slot the runtime reads to answer a chat
 * (`internal/app/runtime.go`'s `models.For`; `subconscious` falls back to
 * it in `continuity.go`), and with it unset `agentloop.Resolve` returns
 * `AOS_AGENT_PROVIDER_NOT_ENABLED` — "no model provider is configured for
 * this agent". So a person could connect a provider, watch the Models
 * list show a model next to "Default", send a message, and get silence:
 * the model shown there was a display-time fallback
 * (`models-section.tsx`'s `resolveSlotValue`) that had never been saved.
 *
 * Seeding here rather than at render time keeps the write tied to
 * something the person actually did, and it only ever fills an *empty*
 * slot — a deliberate choice is never overwritten by connecting another
 * provider.
 *
 * `modelId` is passed in rather than looked up because the caller is the
 * one that can ask the provider (see `firstModelOf`). That is what finally
 * covers `openrouter`/`crof`/`opencode`, whose static catalogues are empty
 * on purpose — hundreds of vendor slugs that change without this build
 * being rebuilt — so connecting one used to leave the slot unset and the
 * agent unable to answer until somebody edited the config by hand.
 */
function seedDefaultSlot(
  models: ConfigAgentModels | undefined,
  providerId: string,
  modelId: string,
): ConfigAgentModels | undefined {
  const current = models?.default;
  if (current?.provider && current?.model) return models;

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
 * Disconnects a provider — the meaning an empty key used to carry.
 */
export async function disconnectModelProvider(id: string): Promise<void> {
  await writeProviders((providers) => removeProvider(providers, id));
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
  editModels?: (models: ConfigAgentModels | undefined) => ConfigAgentModels | undefined,
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
  const models = editModels?.(config?.agents?.models);

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
