import { z } from "zod";
import { Schema } from "@/core/helpers/schema.helper";
import { ThemeSettingsSchema } from "@/features/theme/schemas/theme.schema";

// ============================================================================
// Nested Objects
// Extracted object schemas composed into the config entity
// ============================================================================

/**
 * AOS user profile fields stored in global config.
 *
 * @example
 * ```typescript
 * {
 *   name: "Felipe",
 *   email: "felipe@example.com",
 *   role: "developer"
 * }
 * ```
 */
export const ConfigUserSchema = Schema.object({
  name: z
    .string()
    .default("User")
    .describe('The user\'s display name. Example: "Felipe".'),
  email: z
    .string()
    .describe('The user\'s email. Example: "felipe@example.com".'),
  role: z
    .string()
    .default("developer")
    .describe('The user\'s role. Example: "developer".'),
});

/**
 * Provider connection entry for agent model adapters.
 *
 * @example
 * ```typescript
 * {
 *   id: "openai",
 *   key: "sk-..."
 * }
 * ```
 */
export const ConfigAgentProviderConnectionSchema = Schema.object({
  id: z
    .string()
    .describe('Unique provider identifier. Example: "openai".'),
  key: z
    .string()
    .optional()
    .describe('Optional provider API key. Example: "sk-...".'),
});

/**
 * Reasoning effort level for an agent model slot.
 *
 * @example
 * ```typescript
 * "medium"
 * ```
 */
export const ConfigAgentReasoningSchema = z
  .enum([
    "provider-default",
    "none",
    "minimal",
    "low",
    "medium",
    "high",
    "xhigh",
  ])
  .describe(
    'Reasoning effort level for a model slot. Example: "medium".',
  );

/**
 * Model assignment for one agent capability slot.
 *
 * @example
 * ```typescript
 * {
 *   provider: "openai",
 *   model: "gpt-4o",
 *   reasoning: "medium"
 * }
 * ```
 */
export const ConfigAgentModelSchema = Schema.object({
  provider: z
    .string()
    .describe('Provider ID for this model slot. Example: "openai".'),
  model: z
    .string()
    .describe('Model identifier for this slot. Example: "gpt-4o".'),
  reasoning: ConfigAgentReasoningSchema.default("medium"),
});

/**
 * Model slots used by AOS agents (default, subconscious, media, …).
 *
 * @example
 * ```typescript
 * {
 *   default: { provider: "openai", model: "gpt-4o", reasoning: "medium" },
 *   subconscious: { provider: "openai", model: "gpt-4o", reasoning: "medium" },
 *   realtime: { provider: "", model: "", reasoning: "medium" },
 *   voice: { provider: "", model: "", reasoning: "medium" },
 *   image: { provider: "", model: "", reasoning: "medium" },
 *   video: { provider: "", model: "", reasoning: "medium" }
 * }
 * ```
 */
export const ConfigAgentModelsSchema = Schema.object({
  default: ConfigAgentModelSchema,
  subconscious: ConfigAgentModelSchema,
  realtime: ConfigAgentModelSchema,
  voice: ConfigAgentModelSchema,
  image: ConfigAgentModelSchema,
  video: ConfigAgentModelSchema,
});

/**
 * Agent-related global configuration (providers + model slots).
 *
 * @example
 * ```typescript
 * {
 *   providers: [{ id: "openai", key: "sk-..." }],
 *   models: {
 *     default: { provider: "openai", model: "gpt-4o", reasoning: "medium" }
 *   }
 * }
 * ```
 */
export const ConfigAgentSchema = Schema.object({
  providers: z
    .array(ConfigAgentProviderConnectionSchema)
    .optional()
    .describe(
      'Configured provider connections. Example: [{ "id": "openai" }].',
    ),
  models: ConfigAgentModelsSchema.optional(),
});

/**
 * Locale / location preferences for the AOS user.
 *
 * @example
 * ```typescript
 * {
 *   language: "en-US",
 *   city: "São Paulo",
 *   country: "BR",
 *   timezone: "America/Sao_Paulo"
 * }
 * ```
 */
export const ConfigRegionSchema = Schema.object({
  language: z
    .string()
    .describe('The user\'s preferred language. Example: "en-US".'),
  city: z
    .string()
    .optional()
    .describe('The user\'s city location. Example: "São Paulo".'),
  country: z
    .string()
    .optional()
    .describe('The user\'s country location. Example: "BR".'),
  timezone: z
    .string()
    .optional()
    .describe('The user\'s local timezone. Example: "America/Sao_Paulo".'),
});

/**
 * Appearance theme preset + dark/light settings (UI shell companion shape).
 *
 * @example
 * ```typescript
 * {
 *   preset: "aos",
 *   settings: { dark: { accent: "#fff" }, light: { accent: "#000" } }
 * }
 * ```
 */
export const ConfigAppearanceThemeSchema = Schema.object({
  preset: z
    .string()
    .default("aos")
    .describe('Theme preset identifier. Example: "aos".'),
  settings: Schema.object({
    dark: ThemeSettingsSchema,
    light: ThemeSettingsSchema,
  }),
});

/**
 * Font size preferences for UI and code surfaces.
 *
 * @example
 * ```typescript
 * {
 *   ui: 14,
 *   code: 13
 * }
 * ```
 */
export const ConfigAppearanceFontSizesSchema = Schema.object({
  ui: z.number().describe("UI font size in pixels. Example: 14."),
  code: z.number().describe("Code font size in pixels. Example: 13."),
});

/**
 * Full appearance preferences used by the theme UI store.
 *
 * Not persisted inside {@link ConfigSchema} — kept here as the shared
 * companion DTO for appearance/theme presentation.
 *
 * @example
 * ```typescript
 * {
 *   mode: "dark",
 *   theme: { preset: "aos", settings: { dark: {}, light: {} } },
 *   fontSizes: { ui: 14, code: 13 }
 * }
 * ```
 */
export const ConfigAppearanceSchema = Schema.object({
  mode: z
    .enum(["light", "dark", "system"])
    .describe('UI Mode preference. Example: "dark".'),
  theme: ConfigAppearanceThemeSchema,
  fontSizes: ConfigAppearanceFontSizesSchema,
});

/**
 * General runtime preferences.
 *
 * @example
 * ```typescript
 * {
 *   preventSleep: true
 * }
 * ```
 */
export const ConfigGeneralSchema = Schema.object({
  preventSleep: z
    .boolean()
    .describe(
      "Whether to prevent sleep during long executions. Example: true.",
    ),
});

/**
 * OS notification preference toggle.
 *
 * @example
 * ```typescript
 * {
 *   enabled: true
 * }
 * ```
 */
export const ConfigNotificationsSchema = Schema.object({
  enabled: z
    .boolean()
    .describe(
      "Whether to enable OS notifications for permission requests. Example: true.",
    ),
});

/**
 * Local authentication / API access settings.
 *
 * @example
 * ```typescript
 * {
 *   enabled: false,
 *   password: "",
 *   secret: "",
 *   apiToken: ""
 * }
 * ```
 */
export const ConfigSecuritySchema = Schema.object({
  enabled: z
    .boolean()
    .default(false)
    .describe(
      "Whether authentication is required to access the AOS instance. Example: false.",
    ),
  password: z
    .string()
    .default("")
    .describe(
      "@deprecated Legacy single-user password hash. Kept only so the multi-user migration can promote it into a super account on `users.json`. New installs MUST store passwords per user — never write here.",
    ),
  secret: z
    .string()
    .default("")
    .describe(
      "HMAC secret for JWT token signing (auto-generated on first use). Installation-scoped, not per user. Example: \"a1b2c3...\".",
    ),
  apiToken: z
    .string()
    .default("")
    .describe(
      "@deprecated Legacy installation API token. Kept only so the multi-user migration can copy it onto the bootstrap super account. New installs MUST store `aos_` tokens per user on `users.json`.",
    ),
  multiUserMigrated: z
    .boolean()
    .default(false)
    .describe(
      "Whether the one-time multi-user migration already promoted the legacy config identity into a super account. Example: false.",
    ),
  chatMultiUserMigrated: z
    .boolean()
    .default(false)
    .describe(
      "Whether the one-time chat multi-user migration already backfilled kind/visibility/participants and split agent-slug DMs. Example: false.",
    ),
});

/**
 * Anonymous diagnostics & crash reporting configuration.
 *
 * @example
 * ```typescript
 * {
 *   enabled: true,
 *   identifier: "installation-id"
 * }
 * ```
 */
export const ConfigTelemetrySchema = Schema.object({
  enabled: z
    .boolean()
    .default(true)
    .describe(
      "Whether anonymous error reporting is enabled. Example: true.",
    ),
  identifier: z
    .string()
    .default("")
    .describe(
      "Anonymous random UUID used only to identify this AOS installation. Example: \"installation-id\".",
    ),
});

/**
 * Cloudflare tunnel metadata stored in global config.
 *
 * @example
 * ```typescript
 * {
 *   enabled: false,
 *   hostname: "",
 *   token: "",
 *   provider: "cloudflare"
 * }
 * ```
 */
export const ConfigTunnelSchema = Schema.object({
  enabled: z
    .boolean()
    .default(false)
    .describe("Whether the Cloudflare tunnel is active. Example: false."),
  hostname: z
    .string()
    .default("")
    .describe(
      'The public hostname assigned to the tunnel. Example: "my-workspace.example.com".',
    ),
  token: z
    .string()
    .default("")
    .describe(
      "The Cloudflare tunnel token used to start cloudflared. Example: \"eyJ...\".",
    ),
  provider: z
    .enum(["cloudflare"])
    .default("cloudflare")
    .describe('The tunnel provider. Example: "cloudflare".'),
});

// ============================================================================
// Entity
// Full persisted / domain shape — master blueprint
// ============================================================================

/**
 * Primary schema for the user configuration stored in `~/.aos/config.json`.
 *
 * @example
 * ```typescript
 * {
 *   user: { name: "Felipe", email: "felipe@example.com", role: "developer" },
 *   agents: { models: { default: { provider: "openai", model: "gpt-4o", reasoning: "medium" } } },
 *   region: { language: "en-US" },
 *   general: { preventSleep: true },
 *   notifications: { enabled: true },
 *   security: { enabled: false, password: "", secret: "", apiToken: "" },
 *   telemetry: { enabled: true, identifier: "" },
 *   tunnel: { enabled: false, hostname: "", token: "", provider: "cloudflare" }
 * }
 * ```
 */
export const ConfigSchema = Schema.object({
  user: ConfigUserSchema,
  agents: ConfigAgentSchema,
  region: ConfigRegionSchema,
  general: ConfigGeneralSchema,
  notifications: ConfigNotificationsSchema,
  security: ConfigSecuritySchema,
  telemetry: ConfigTelemetrySchema,
  tunnel: ConfigTunnelSchema,
});

// ============================================================================
// Get
// Path/query-less input (no route params)
// ============================================================================

/**
 * Get-config input — empty object; get has no Body/Query/Params split.
 *
 * Controllers bind GET with no additional transport schema; domain layers may
 * still type against this SSOT for symmetry with other entities.
 *
 * @example
 * ```typescript
 * {}
 * ```
 */
export const ConfigGetInputSchema = Schema.object({});

// ============================================================================
// Update
// Input derived from entity (partial patch)
// ============================================================================

/**
 * Update-config input — all top-level sections optional for deep-merge patches.
 *
 * Controllers bind HTTP body to this schema; services deep-merge against defaults.
 *
 * @example
 * ```typescript
 * {
 *   region: { timezone: "America/Sao_Paulo" },
 *   notifications: { enabled: false }
 * }
 * ```
 */
export const ConfigUpdateInputSchema = ConfigSchema.extend({
  user: ConfigUserSchema.partial(),
  agents: ConfigAgentSchema.partial(),
  region: ConfigRegionSchema.partial(),
  general: ConfigGeneralSchema.partial(),
  notifications: ConfigNotificationsSchema.partial(),
  security: ConfigSecuritySchema.partial(),
  telemetry: ConfigTelemetrySchema.partial(),
  tunnel: ConfigTunnelSchema.partial(),
}).partial();

/**
 * @deprecated Prefer {@link ConfigUpdateInputSchema}.
 */
export const ConfigUpdateSchema = ConfigUpdateInputSchema;

/**
 * Default config document used for first-run writes and deep-merge normalization.
 *
 * Seeds `~/.aos/config.json` when missing and acts as the left-hand baseline for
 * {@link DeepMergeHelper.merge} when repairing partial bootstrap files.
 *
 * @example
 * ```typescript
 * {
 *   user: { name: "", email: "", role: "" },
 *   region: { language: "en-US" },
 *   telemetry: { enabled: true, identifier: "" }
 * }
 * ```
 */
export const AOS_DEFAULT_CONFIG: z.infer<typeof ConfigSchema> = {
  user: { name: "", role: "", email: "" },
  agents: {
    models: {
      default: { provider: "", model: "", reasoning: "medium" },
      subconscious: { provider: "", model: "", reasoning: "medium" },
      realtime: { provider: "", model: "", reasoning: "medium" },
      voice: { provider: "", model: "", reasoning: "medium" },
      image: { provider: "", model: "", reasoning: "medium" },
      video: { provider: "", model: "", reasoning: "medium" },
    },
  },
  general: { preventSleep: true },
  notifications: { enabled: true },
  region: { language: "en-US" },
  security: {
    enabled: false,
    password: "",
    secret: "",
    apiToken: "",
    multiUserMigrated: false,
    chatMultiUserMigrated: false,
  },
  telemetry: {
    enabled: true,
    identifier: "",
  },
  tunnel: {
    enabled: false,
    hostname: "",
    token: "",
    provider: "cloudflare",
  },
};
