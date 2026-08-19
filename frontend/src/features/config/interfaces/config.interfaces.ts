import type { z } from "zod";
import type { ResponseWithCTA } from "@/core/interfaces/response.interfaces";
import type { RequestActor } from "@/core/helpers/request-context";
import type { FractalActivityInstance } from "@/core/services/activity";
import type {
  FractalConfigAgentModelSchema,
  FractalConfigAgentModelsSchema,
  FractalConfigAgentProviderConnectionSchema,
  FractalConfigAgentReasoningSchema,
  FractalConfigAgentSchema,
  FractalConfigAppearanceFontSizesSchema,
  FractalConfigAppearanceSchema,
  FractalConfigAppearanceThemeSchema,
  FractalConfigGeneralSchema,
  FractalConfigGetInputSchema,
  FractalConfigNotificationsSchema,
  FractalConfigRegionSchema,
  FractalConfigSchema,
  FractalConfigSecuritySchema,
  FractalConfigTelemetrySchema,
  FractalConfigTunnelSchema,
  FractalConfigUpdateInputSchema,
  FractalConfigUserSchema,
} from "../schemas/config.schema";

// ============================================================================
// Enums
// Inferred enum types for Config
// ============================================================================

/** {@inheritDoc FractalConfigAgentReasoningSchema} */
export type FractalConfigAgentReasoning = z.infer<
  typeof FractalConfigAgentReasoningSchema
>;

// ============================================================================
// Nested Objects
// Inferred nested object types
// ============================================================================

/** {@inheritDoc FractalConfigUserSchema} */
export type FractalConfigUser = z.infer<typeof FractalConfigUserSchema>;

/** {@inheritDoc FractalConfigAgentProviderConnectionSchema} */
export type FractalConfigAgentProviderConnection = z.infer<
  typeof FractalConfigAgentProviderConnectionSchema
>;

/** {@inheritDoc FractalConfigAgentModelSchema} */
export type FractalConfigAgentModel = z.infer<
  typeof FractalConfigAgentModelSchema
>;

/** {@inheritDoc FractalConfigAgentModelsSchema} */
export type FractalConfigAgentModels = z.infer<
  typeof FractalConfigAgentModelsSchema
>;

/** {@inheritDoc FractalConfigAgentSchema} */
export type FractalConfigAgent = z.infer<typeof FractalConfigAgentSchema>;

/** {@inheritDoc FractalConfigRegionSchema} */
export type FractalConfigRegion = z.infer<typeof FractalConfigRegionSchema>;

/** {@inheritDoc FractalConfigAppearanceThemeSchema} */
export type FractalConfigAppearanceTheme = z.infer<
  typeof FractalConfigAppearanceThemeSchema
>;

/** {@inheritDoc FractalConfigAppearanceFontSizesSchema} */
export type FractalConfigAppearanceFontSizes = z.infer<
  typeof FractalConfigAppearanceFontSizesSchema
>;

/** {@inheritDoc FractalConfigAppearanceSchema} */
export type FractalConfigAppearance = z.infer<
  typeof FractalConfigAppearanceSchema
>;

/** {@inheritDoc FractalConfigGeneralSchema} */
export type FractalConfigGeneral = z.infer<typeof FractalConfigGeneralSchema>;

/** {@inheritDoc FractalConfigNotificationsSchema} */
export type FractalConfigNotifications = z.infer<
  typeof FractalConfigNotificationsSchema
>;

/** {@inheritDoc FractalConfigSecuritySchema} */
export type FractalConfigSecurity = z.infer<typeof FractalConfigSecuritySchema>;

/** {@inheritDoc FractalConfigTelemetrySchema} */
export type FractalConfigTelemetry = z.infer<
  typeof FractalConfigTelemetrySchema
>;

/** {@inheritDoc FractalConfigTunnelSchema} */
export type FractalConfigTunnel = z.infer<typeof FractalConfigTunnelSchema>;

// ============================================================================
// Entity
// Full entity type
// ============================================================================

/** {@inheritDoc FractalConfigSchema} */
export type FractalConfig = z.infer<typeof FractalConfigSchema>;

/**
 * Default config.json payload used for first-run bootstrap and DeepMerge bases.
 */
export const FRACTAL_DEFAULT_CONFIG: FractalConfig = {
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

/** {@inheritDoc FractalConfigGetInputSchema} */
export type FractalConfigGetInput = z.infer<typeof FractalConfigGetInputSchema>;

/**
 * Get-config service result — installation status plus the persisted config.
 *
 * `status` is a virtual field (`waiting` | `done`) derived from whether Fractal
 * has completed first-run setup. Used by the frontend boot middleware to decide
 * between `/onboarding` and `/login`.
 *
 * Returned by {@link IFractalConfigService.get}.
 */
export type FractalConfigGetResult = ResponseWithCTA<{
  /** Whether first-run onboarding is still required. */
  status: "waiting" | "done";
  /** Full persisted global configuration document. */
  config: FractalConfig;
}>;

// ============================================================================
// Update
// Input + Result
// ============================================================================

/** {@inheritDoc FractalConfigUpdateInputSchema} */
export type FractalConfigUpdateInput = z.infer<
  typeof FractalConfigUpdateInputSchema
>;

/**
 * @deprecated Prefer {@link FractalConfigUpdateInput}.
 */
export type FractalConfigUpdate = FractalConfigUpdateInput;

/**
 * Update-config service result — named payload wrapped with CTA commands.
 *
 * Returned by {@link IFractalConfigService.update}.
 */
export type FractalConfigUpdateResult = ResponseWithCTA<{
  config: FractalConfig;
}>;

// ============================================================================
// Activity Events
// Catalog emit types inferred from FractalActivityInstance (activity.ts SSOT)
// ============================================================================

/**
 * Notify contracts for the `config` activity namespace.
 *
 * Mapped from {@link FractalActivityInstance} at `events.config`.
 */
export type FractalConfigActivityEvents =
  FractalActivityInstance["events"]["config"];

/**
 * Literal event keys registered under `activity.events.config`.
 *
 * @example
 * ```typescript
 * const event: FractalConfigActivityEvent = "updated";
 * ```
 */
export type FractalConfigActivityEvent = keyof FractalConfigActivityEvents;

/**
 * Full notify payload for a config activity event (includes `causer`).
 *
 * @typeParam E - Event key under `activity.events.config`.
 */
export type FractalConfigActivityEventData<
  E extends FractalConfigActivityEvent,
> = Parameters<FractalConfigActivityEvents[E]["notify"]>[0];

/**
 * Emit payload for a config activity event — **without** `causer`.
 *
 * Nested under {@link FractalConfigEmitInput.data}. Pass `causer` on the
 * input object when ambient {@link RequestContext} is unavailable.
 *
 * @typeParam E - Event key under `activity.events.config`.
 */
export type FractalConfigEmitData<E extends FractalConfigActivityEvent> = Omit<
  FractalConfigActivityEventData<E>,
  "causer"
>;

/**
 * Single params object for {@link IFractalConfigService.emit}.
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
export type FractalConfigEmitInput<E extends FractalConfigActivityEvent> = {
  /** Catalog event name (literal key under `activity.events.config`). */
  event: E;
  /** Event payload without `causer`. */
  data: FractalConfigEmitData<E>;
  /** Optional actor; omit to use ambient RequestContext. */
  causer?: RequestActor;
};
