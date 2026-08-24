import * as React from "react";
import { aos } from "@/app/aos";
import type { ModelProviderOption } from "@/features/model/interfaces/model.interfaces";

/**
 * The catalogue, asked rather than declared.
 *
 * `models_list` (`internal/domain/model/commands.go`) calls each connected
 * provider's own catalogue endpoint with the credential this installation
 * holds, so what comes back is what that account can actually reach today.
 * That is the whole point: every hardcoded list in this build has been wrong
 * at least once — the Codex entries named a model that does not exist and
 * omitted three that do, and the Google ones had to be corrected twice against
 * the live API before any turn could answer at all.
 *
 * The static `PROVIDER_CATALOG` is not replaced by this. It still owns what a
 * provider *is* — its name, its description, how it authenticates — none of
 * which a provider publishes and none of which changes when OpenAI ships a
 * model. It also stays the fallback for the case this cannot cover: a provider
 * that is not connected yet has no credential to ask with, so its models can
 * only be the ones written here.
 */

/** One provider's answer, as Go's `model.Provider` serializes it. */
interface DiscoveredProvider {
  id?: string;
  models?: { id?: string; name?: string }[];
  /** Why this provider's catalogue is empty, when asking failed. */
  error?: string;
}

interface ListModelsOutput {
  providers?: DiscoveredProvider[];
  total?: number;
}

/** What a caller gets: models by provider id, and the reasons for the gaps. */
export interface ModelDiscovery {
  models: Map<string, ModelProviderOption[]>;
  errors: Map<string, string>;
  /** True while the first answer is still in flight. */
  pending: boolean;
}

const EMPTY: ModelDiscovery = { models: new Map(), errors: new Map(), pending: false };

/**
 * How long an answer is reused before asking again.
 *
 * The daemon caches for five minutes for the same reason (see
 * `internal/app/models.go`); this keeps a re-render from queueing a request
 * that would only be served from that cache anyway. Connecting or
 * disconnecting a provider does not wait for either: the call sites invalidate
 * this query, because "I just pasted my key" is exactly when a stale answer is
 * least welcome.
 */
const STALE_MS = 5 * 60 * 1000;

/** The query key, so a call site can invalidate it after changing a credential. */
export const MODEL_DISCOVERY_KEY = ["model", "list"] as const;

export function useDiscoveredModels(enabled: boolean): ModelDiscovery {
  const query = aos.client.model.list.useQuery<ListModelsOutput | null>({
    enabled,
    staleTime: STALE_MS,
  });

  const data = query.data;
  const pending = enabled && query.isPending;

  return React.useMemo(() => {
    if (!data?.providers) return { ...EMPTY, pending };

    const models = new Map<string, ModelProviderOption[]>();
    const errors = new Map<string, string>();

    for (const provider of data.providers) {
      if (!provider?.id) continue;
      if (provider.error) errors.set(provider.id, provider.error);

      const options = (provider.models ?? [])
        .filter((m): m is { id: string; name?: string } => !!m?.id)
        // `enabled` is this build's own notion — a model a person switched
        // off — and nothing a provider publishes. Everything it serves is on.
        //
        // `capabilities` is left unset deliberately, which is what keeps the
        // realtime/voice/image/video slots honestly empty: no adapter here
        // implements anything past text and tool calls, so claiming a
        // capability would offer a combination that fails the moment it runs.
        .map((m) => ({ id: m.id, name: m.name || m.id, enabled: true }));

      if (options.length > 0) models.set(provider.id, options);
    }
    return { models, errors, pending };
  }, [data, pending]);
}
