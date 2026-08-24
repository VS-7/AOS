import { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import { LanguageModel } from "ai";
import z from "zod";

export const ListModelsSchena = z.object({
  mode: z.enum(['available', 'enabled']).optional().default('enabled'),
  provider: z.string().optional()
})

export const InstallModelSchema = z.object({
  id: z.string(),
  key: z.string().optional().default(""),
  models: z.array(z.string()),
  default: z.boolean().optional()
})

export type ListModelsInput = Partial<z.infer<typeof ListModelsSchena>>
export type InstalModelInput = z.infer<typeof InstallModelSchema>

/**
 * `capabilities` — absent from this file's source; reconstructed from
 * `presentation/components/settings/.../models-section.tsx`'s
 * `supportsCapability`/`getFirstModel` helpers, its only consumer.
 */
export interface ModelProviderOptionCapabilities {
  fast?: boolean;
  reasoning?: boolean;
  vision?: boolean;
  realtime?: boolean;
  voice?: boolean;
  image?: boolean;
  video?: boolean;
}

export interface ModelProviderOption {
  id: string;
  name: string;
  enabled: boolean;
  capabilities?: ModelProviderOptionCapabilities;
}

export interface ModelProvider {
  id: string;
  name: string;
  description: string;
  // `{ light, dark }`, not a single URL — reconstructed from
  // `hooks/use-provider-logo.ts`, this field's only consumer, which picks
  // one per the active theme mode.
  logo: { light: string; dark: string };
  configured: boolean;
  default: boolean;
  auth: ModelProviderAuth;
  models: ModelProviderOption[]
  /**
   * Whether `models` is what the provider itself answered, or the static
   * fallback in `data/provider-catalog.ts`.
   *
   * Not decoration: the difference is between a list that is true right now
   * and one that was true when somebody typed it. A screen that shows both
   * identically gives a person no way to tell that the provider was never
   * actually reached — the exact silence this project has already paid for
   * once, in the chat.
   */
  modelsDiscovered?: boolean;
  /** Why the provider could not be asked, when asking failed. */
  modelsError?: string;
}

export interface ModelProviderAuth {
  mode: "api-key" | "oauth-file" | "cache-dir";
  connectionType: "external" | "local";
  label: string;
  placeholder: string;
  description: string;
  required: boolean;
  masked?: boolean;
}

export interface IModelProviderAdapter {
  id: string;
  name: string;
  description: string;
  logo: string;
  auth: ModelProviderAuth;

  /**
   * Validate the provided API key.
   */
  validate(key: string): Promise<boolean>;

  /**
   * List available models for this provider.
   */
  models(key?: string): Promise<Omit<ModelProviderOption, 'enabled'>[]>;

  /**
   * Initialize a LanguageModel instance.
   */
  init(modelId: string, key?: string): Promise<LanguageModel>;
}

export interface IModelProviderService {
  list(params?: ListModelsInput): Promise<ModelProvider[]>;
  get(id: string): Promise<IModelProviderAdapter | null>
  set(params: InstalModelInput): Promise<ResponseWithCTA>;
}
