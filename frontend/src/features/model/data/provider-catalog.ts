import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";

/**
 * Static reference data for the providers `internal/runtime/providers`
 * actually registers — `internal/runtime/providers/{anthropic,openai,
 * google,compat}/*.go`'s own `providers.Register` calls, verified by grep,
 * not assumed. AOS's Go backend has no live "list models for this
 * provider" command (the original's `IModelProviderAdapter.models()`
 * called each provider's real `/models` endpoint; nothing here does), so
 * this is a curated, static list rather than a discovered one. The model
 * ids for anthropic/openai/google are the same ones `internal/runtime/
 * providers/pricing.json` already prices, so a model picked here is
 * guaranteed to be one the backend already knows the cost of.
 *
 * `capabilities` is left unset on every model: none of the four provider
 * adapters implement anything beyond `Generate`/`Stream` (plain text and
 * tool-calling) today, so no model here actually backs the realtime/voice/
 * image/video slots. Leaving those flags unset means those slots correctly
 * show "nothing configured supports this" instead of offering a
 * combination that would fail loudly the moment it's used
 * (`providers.Capability`'s own `errCapabilityNotSupported`).
 *
 * `logo` stays `{light: "", dark: ""}` for every entry: no logo asset ships
 * with this build, and `useProviderLogo`'s consumers already treat an
 * empty src as "render nothing" — not a regression, since a logo never
 * rendered here before this catalog existed either.
 *
 * `openrouter`/`crof`/`opencode` get an empty `models` list on purpose:
 * they're OpenAI-compatible gateways whose real catalogs span hundreds of
 * vendor/model slugs that change independently of this build. Guessing a
 * handful would be worse than leaving the picker honestly empty until
 * real catalog discovery exists.
 */
export type ProviderCatalogEntry = Omit<ModelProvider, "configured">;

function apiKeyAuth(opts: {
  placeholder: string;
  description: string;
  required?: boolean;
}): ModelProvider["auth"] {
  return {
    mode: "api-key",
    connectionType: "external",
    label: "API Key",
    placeholder: opts.placeholder,
    description: opts.description,
    required: opts.required ?? true,
    masked: true,
  };
}

function oauthFileAuth(path: string, tool: string): ModelProvider["auth"] {
  return {
    mode: "oauth-file",
    connectionType: "local",
    label: "Key (optional override)",
    placeholder: "",
    description: `Uses the ${tool} login already on this machine (${path}) — nothing to enter here unless you want to override it with an API key.`,
    required: false,
    masked: true,
  };
}

export const PROVIDER_CATALOG: ProviderCatalogEntry[] = [
  {
    id: "anthropic",
    name: "Anthropic",
    description: "Claude models, direct from Anthropic.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "sk-ant-...",
      description: "Visit Anthropic's console to get your API key.",
    }),
    // Opus 4.5 and Sonnet 4.5 are superseded; the current ids carry no
    // date suffix. Haiku 4.5 is still current and unchanged.
    models: [
      { id: "claude-opus-5", name: "Claude Opus 5", enabled: true },
      { id: "claude-sonnet-5", name: "Claude Sonnet 5", enabled: true },
      { id: "claude-haiku-4-5", name: "Claude Haiku 4.5", enabled: true },
    ],
  },
  {
    id: "openai",
    name: "OpenAI",
    description: "GPT and o-series models, direct from OpenAI.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "sk-...",
      description: "Visit the OpenAI platform to get your API key.",
    }),
    models: [
      { id: "gpt-5.1", name: "GPT-5.1", enabled: true },
      { id: "gpt-5.1-mini", name: "GPT-5.1 Mini", enabled: true },
      { id: "gpt-5.1-nano", name: "GPT-5.1 Nano", enabled: true },
      { id: "o3", name: "o3", enabled: true },
    ],
  },
  {
    id: "codex",
    name: "ChatGPT (Codex login)",
    description: "The same OpenAI models, billed to a ChatGPT subscription instead of API usage.",
    logo: { light: "", dark: "" },
    default: false,
    auth: oauthFileAuth("~/.codex/auth.json", "Codex CLI / ChatGPT desktop app"),
    // The Codex surface has its own model line, and it is not the public
    // API's — these are the four the original's own adapter lists
    // (`_extracted/v401/server/.../adapters/codex.adapter.ts`), confirmed
    // against the endpoint. `gpt-5.1` was never one of them.
    models: [
      { id: "gpt-5.4", name: "GPT-5.4", enabled: true },
      { id: "gpt-5.5", name: "GPT-5.5", enabled: true },
      { id: "gpt-5.4-mini", name: "GPT-5.4 Mini", enabled: true },
      { id: "gpt-5.3-codex-spark", name: "GPT-5.3 Codex Spark", enabled: true },
    ],
  },
  {
    id: "google",
    name: "Google",
    description: "Gemini models, direct from Google.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "AIza...",
      description: "Visit Google AI Studio to get your API key.",
    }),
    // Verified against this API's own ListModels, not composed from a
    // version number: `gemini-3-pro` and `gemini-3-flash` were never real
    // ids, and generateContent answered 404 "is not found for API version
    // v1beta" for both — every Google turn failed for that reason alone,
    // with a valid key. Flash is first because the first entry is what
    // `seedDefaultSlot` picks when connecting the provider, and it is the
    // one confirmed to answer.
    models: [
      { id: "gemini-3-flash-preview", name: "Gemini 3 Flash", enabled: true },
      { id: "gemini-3.1-pro-preview", name: "Gemini 3.1 Pro", enabled: true },
    ],
  },
  {
    id: "gemini-cli",
    name: "Gemini (CLI login)",
    description: "The same Gemini models, billed to a Gemini CLI allowance instead of API usage.",
    logo: { light: "", dark: "" },
    default: false,
    auth: oauthFileAuth("~/.gemini/oauth_creds.json", "Gemini CLI"),
    models: [{ id: "gemini-3-flash-preview", name: "Gemini 3 Flash", enabled: true }],
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    description: "A gateway to hundreds of models from many vendors, billed through one OpenRouter key.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "sk-or-...",
      description: "Visit OpenRouter to get your API key.",
    }),
    models: [],
  },
  {
    id: "crof",
    name: "Crof",
    description: "An OpenAI-compatible gateway, billed through one Crof key.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "API key",
      description: "Visit Crof to get your API key.",
      required: false,
    }),
    models: [],
  },
  {
    id: "opencode",
    name: "opencode Zen",
    description: "opencode's own model gateway. Models whose id ends in “-free” need no key.",
    logo: { light: "", dark: "" },
    default: false,
    auth: apiKeyAuth({
      placeholder: "API key",
      description: "Optional — required only for models that aren't free-tier.",
      required: false,
    }),
    models: [],
  },
];
