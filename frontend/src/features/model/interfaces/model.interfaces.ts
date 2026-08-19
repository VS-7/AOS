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
export interface FractalModelProviderOptionCapabilities {
  fast?: boolean;
  reasoning?: boolean;
  vision?: boolean;
  realtime?: boolean;
  voice?: boolean;
  image?: boolean;
  video?: boolean;
}

export interface FractalModelProviderOption {
  id: string;
  name: string;
  enabled: boolean;
  capabilities?: FractalModelProviderOptionCapabilities;
}

export interface FractalModelProvider {
  id: string;
  name: string;
  description: string;
  // `{ light, dark }`, not a single URL — reconstructed from
  // `hooks/use-provider-logo.ts`, this field's only consumer, which picks
  // one per the active theme mode.
  logo: { light: string; dark: string };
  configured: boolean;
  default: boolean;
  auth: FractalModelProviderAuth;
  models: FractalModelProviderOption[]
}

export interface FractalModelProviderAuth {
  mode: "api-key" | "oauth-file" | "cache-dir";
  connectionType: "external" | "local";
  label: string;
  placeholder: string;
  description: string;
  required: boolean;
  masked?: boolean;
}

export interface IFractalModelProviderAdapter {
  id: string;
  name: string;
  description: string;
  logo: string;
  auth: FractalModelProviderAuth;

  /**
   * Validate the provided API key.
   */
  validate(key: string): Promise<boolean>;

  /**
   * List available models for this provider.
   */
  models(key?: string): Promise<Omit<FractalModelProviderOption, 'enabled'>[]>;

  /**
   * Initialize a LanguageModel instance.
   */
  init(modelId: string, key?: string): Promise<LanguageModel>;
}

export interface IFractalModelProviderService {
  list(params?: ListModelsInput): Promise<FractalModelProvider[]>;
  get(id: string): Promise<IFractalModelProviderAdapter | null>
  set(params: InstalModelInput): Promise<ResponseWithCTA>;
}
