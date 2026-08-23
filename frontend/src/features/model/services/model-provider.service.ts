import * as React from "react";
import { aos } from "@/app/aos";
import { PROVIDER_CATALOG } from "@/features/model/data/provider-catalog";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import type { Config, ConfigAgentProviderConnection } from "@/features/config/interfaces/config.interfaces";
import { mergeProviderKey } from "./merge-provider-key";

export { mergeProviderKey };

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
        configured: providers.some((p) => p.id === entry.id && !!p.key?.trim()),
      })),
    [providers],
  );
}

/**
 * Sets (or, with an empty/whitespace key, clears) one provider's credential.
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
  const current = await aos.client.config.get.query({
    query: { reveal: true },
  });
  if (current.error) {
    throw new Error(
      current.error.message ?? "Unable to read the current configuration.",
    );
  }

  const config = current.data as Config | undefined;
  const providers = mergeProviderKey(
    config?.agents?.providers ?? [],
    id,
    key,
  );

  const result = await aos.client.config.update.mutate({
    body: { agents: { providers } },
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
