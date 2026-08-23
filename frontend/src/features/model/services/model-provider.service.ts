import * as React from "react";
import { aos } from "@/app/aos";
import { PROVIDER_CATALOG } from "@/features/model/data/provider-catalog";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import type { Config, ConfigAgentModels, ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";
import { mergeProviderKey, removeProvider } from "./merge-provider-key";

export { mergeProviderKey, removeProvider };

/**
 * Replaces the dormant `model.list`/`model.set` commands
 * (`frontend/src/lib/command-map.ts` declares both `null` — `model` stays
 * in `DORMANT_DOMAINS` — because no Go command group for provider/API-key
 * management exists in this rebuild at all, verified by grep, not
 * assumed). What Go does have is `agents.providers`/`agents.models` on
 * `Config` itself (`internal/domain/config/entity.go`), already reachable
 * through the real `config.get`/`config.update`. This module is the seam:
 * a static catalog (`provider-catalog.ts`) for what a provider *is*,
 * merged with the live, already-loaded `Config` for what's actually
 * connected — no separate network round trip for the read side.
 */

/** Every known provider, `configured` filled in from the live config already in route context. */
export function useModelProviders(): ModelProvider[] {
  const context = aos.useContext();
  const providers = (context.config?.agents?.providers ??
    []) as ConfigAgentProviderConnection[];

  return React.useMemo(
    () =>
      PROVIDER_CATALOG.map((entry) => ({
        ...entry,
        // Presence of the entry is what "connected" means, not the key
        // being non-empty. An `oauth-file` provider (`codex`,
        // `gemini-cli`) is connected with no key at all — its credential
        // is another tool's file on this machine. Requiring a non-empty
        // key here meant those two could never show as connected even
        // once their entry was saved, so they stayed in the "Connect"
        // menu forever. An `api-key` provider can't reach this state:
        // its dialog refuses to submit a blank key.
        configured: providers.some((p) => p.id === entry.id),
      })),
    [providers],
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
  await writeProviders(
    (providers) => mergeProviderKey(providers, id, key),
    (models) => seedDefaultSlot(models, id),
  );
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
 * something the person actually did. It only ever fills an *empty* slot —
 * a deliberate choice is never overwritten by connecting another
 * provider — and it stays out of the way for a provider whose catalogue
 * this build doesn't know (`openrouter`/`crof`/`opencode` ship no model
 * list), where any pick would be a guess.
 */
function seedDefaultSlot(
  models: ConfigAgentModels | undefined,
  providerId: string,
): ConfigAgentModels | undefined {
  const current = models?.default;
  if (current?.provider && current?.model) return models;

  const firstModel = PROVIDER_CATALOG.find((p) => p.id === providerId)?.models[0];
  if (!firstModel) return models;

  return {
    ...(models ?? {}),
    default: {
      provider: providerId,
      model: firstModel.id,
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
