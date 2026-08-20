import type { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import type { RequestActor } from "@/core/helpers/request-context";
import type { ActivityInstance } from "@/core/services/activity";
import type {
  ConfigAgentModelSchema,
  ConfigAgentModelsSchema,
  ConfigAgentProviderConnectionSchema,
  ConfigAgentReasoningSchema,
  ConfigAgentSchema,
  ConfigAppearanceFontSizesSchema,
  ConfigAppearanceSchema,
  ConfigAppearanceThemeSchema,
  ConfigGeneralSchema,
  ConfigGetInputSchema,
  ConfigNotificationsSchema,
  ConfigRegionSchema,
  ConfigSchema,
  ConfigSecuritySchema,
  ConfigTelemetrySchema,
  ConfigTunnelSchema,
  ConfigUpdateInputSchema,
  ConfigUserSchema,
} from "../schemas/config.schema";

// ============================================================================
// Enums
// Inferred enum types for Config
// ============================================================================

/** {@inheritDoc ConfigAgentReasoningSchema} */
export type ConfigAgentReasoning = z.infer<
  typeof ConfigAgentReasoningSchema
>;

// ============================================================================
// Nested Objects
// Inferred nested object types
// ============================================================================

/** {@inheritDoc ConfigUserSchema} */
export type ConfigUser = z.infer<typeof ConfigUserSchema>;

/** {@inheritDoc ConfigAgentProviderConnectionSchema} */
export type ConfigAgentProviderConnection = z.infer<
  typeof ConfigAgentProviderConnectionSchema
>;

/** {@inheritDoc ConfigAgentModelSchema} */
export type ConfigAgentModel = z.infer<
  typeof ConfigAgentModelSchema
>;

/** {@inheritDoc ConfigAgentModelsSchema} */
export type ConfigAgentModels = z.infer<
  typeof ConfigAgentModelsSchema
>;

/** {@inheritDoc ConfigAgentSchema} */
export type ConfigAgent = z.infer<typeof ConfigAgentSchema>;

/** {@inheritDoc ConfigRegionSchema} */
export type ConfigRegion = z.infer<typeof ConfigRegionSchema>;

/** {@inheritDoc ConfigAppearanceThemeSchema} */
export type ConfigAppearanceTheme = z.infer<
  typeof ConfigAppearanceThemeSchema
>;

/** {@inheritDoc ConfigAppearanceFontSizesSchema} */
export type ConfigAppearanceFontSizes = z.infer<
  typeof ConfigAppearanceFontSizesSchema
>;

/** {@inheritDoc ConfigAppearanceSchema} */
export type ConfigAppearance = z.infer<
  typeof ConfigAppearanceSchema
>;

/** {@inheritDoc ConfigGeneralSchema} */
export type ConfigGeneral = z.infer<typeof ConfigGeneralSchema>;

/** {@inheritDoc ConfigNotificationsSchema} */
export type ConfigNotifications = z.infer<
  typeof ConfigNotificationsSchema
>;

/** {@inheritDoc ConfigSecuritySchema} */
export type ConfigSecurity = z.infer<typeof ConfigSecuritySchema>;

/** {@inheritDoc ConfigTelemetrySchema} */
export type ConfigTelemetry = z.infer<
  typeof ConfigTelemetrySchema
>;

/** {@inheritDoc ConfigTunnelSchema} */
export type ConfigTunnel = z.infer<typeof ConfigTunnelSchema>;

// ============================================================================
// Entity
// Full entity type
// ============================================================================

/** {@inheritDoc ConfigSchema} */
export type Config = z.infer<typeof ConfigSchema>;

/**
 * Default config.json payload used for first-run bootstrap and DeepMerge bases.
 */
export const AOS_DEFAULT_CONFIG: Config = {
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
  tunnel: { enabled: false, hostname: "", token: "", provider: "cloudflare" },
};

// ============================================================================
// Get
// Input + Result
// ============================================================================

/** {@inheritDoc ConfigGetInputSchema} */
export type ConfigGetInput = z.infer<typeof ConfigGetInputSchema>;

/**
 * Get-config service result — installation status plus the persisted config.
 *
 * `status` is a virtual field (`waiting` | `done`) derived from whether AOS
 * has completed first-run setup. Used by the frontend boot middleware to decide
 * between `/onboarding` and `/login`.
 *
 * Returned by {@link IConfigService.get}.
 */
export type ConfigGetResult = ResponseWithCTA<{
  /** Whether first-run onboarding is still required. */
  status: "waiting" | "done";
  /** Full persisted global configuration document. */
  config: Config;
}>;

// ============================================================================
// Update
// Input + Result
// ============================================================================

/** {@inheritDoc ConfigUpdateInputSchema} */
export type ConfigUpdateInput = z.infer<
  typeof ConfigUpdateInputSchema
>;

/**
 * @deprecated Prefer {@link ConfigUpdateInput}.
 */
export type ConfigUpdate = ConfigUpdateInput;

/**
 * Update-config service result — named payload wrapped with CTA commands.
 *
 * Returned by {@link IConfigService.update}.
 */
export type ConfigUpdateResult = ResponseWithCTA<{
  config: Config;
}>;

// ============================================================================
// Activity Events
// Catalog emit types inferred from ActivityInstance (activity.ts SSOT)
// ============================================================================

/**
 * Notify contracts for the `config` activity namespace.
 *
 * Mapped from {@link ActivityInstance} at `events.config`.
 */
export type ConfigActivityEvents =
  ActivityInstance["events"]["config"];

/**
 * Literal event keys registered under `activity.events.config`.
 *
 * @example
 * ```typescript
 * const event: ConfigActivityEvent = "updated";
 * ```
 */
export type ConfigActivityEvent = keyof ConfigActivityEvents;

/**
 * Full notify payload for a config activity event (includes `causer`).
 *
 * @typeParam E - Event key under `activity.events.config`.
 */
export type ConfigActivityEventData<
  E extends ConfigActivityEvent,
> = Parameters<ConfigActivityEvents[E]["notify"]>[0];

/**
 * Emit payload for a config activity event — **without** `causer`.
 *
 * Nested under {@link ConfigEmitInput.data}. Pass `causer` on the
 * input object when ambient {@link RequestContext} is unavailable.
 *
 * @typeParam E - Event key under `activity.events.config`.
 */
export type ConfigEmitData<E extends ConfigActivityEvent> = Omit<
  ConfigActivityEventData<E>,
  "causer"
>;

/**
 * Single params object for {@link IConfigService.emit}.
 *
 * `causer` is optional — when omitted, `emit` resolves ambient identity via
 * {@link RequestContext.getActorWithThrow}.
 *
 * @typeParam E - Event key under `activity.events.config`.
 *
 * @example
 * ```typescript
 * await config.emit({ event: "updated", data: { section: "region" } });
 * ```
 */
export type ConfigEmitInput<E extends ConfigActivityEvent> = {
  /** Catalog event name (literal key under `activity.events.config`). */
  event: E;
  /** Event payload without `causer`. */
  data: ConfigEmitData<E>;
  /** Optional actor; omit to use ambient RequestContext. */
  causer?: RequestActor;
};
