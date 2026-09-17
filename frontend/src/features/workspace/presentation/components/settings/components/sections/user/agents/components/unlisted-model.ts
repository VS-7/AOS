import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";

/** A slot's saved choice: which provider, and which of its models. */
interface SlotChoice {
  provider: string;
  model: string;
}

/**
 * The model a slot is saved with that its provider no longer lists, or null.
 *
 * A provider can stop serving a model to an account — Codex stopped offering
 * gpt-5.4-mini to ChatGPT logins — and the slot went on showing it as if
 * nothing had changed, so every agent resolving to it failed every turn with
 * nothing here to say why. Only the provider's own answer counts as evidence:
 * the static fallback is what was true when somebody typed it, and a
 * catalogue that failed to load says nothing about any model.
 */
export function unlistedModel(
  providers: ModelProvider[],
  current: SlotChoice,
): string | null {
  if (!current.provider || !current.model) return null;
  const provider = providers.find((p) => p.id === current.provider);
  if (!provider?.configured || !provider.modelsDiscovered || provider.modelsError) return null;
  return provider.models.some((m) => m.id === current.model) ? null : current.model;
}
