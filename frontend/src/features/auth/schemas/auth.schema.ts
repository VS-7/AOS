import { z } from "zod";
import { Schema } from "@/core/helpers/schema.helper";

// ============================================================================
// Login
// Input for email + password authentication
// ============================================================================

/**
 * JSON body for authenticating with email and password.
 *
 * The password is plain text and is verified against the stored argon2id hash.
 *
 * @example
 * ```typescript
 * { email: "user@example.com", password: "my-secret-password" }
 * ```
 */
export const AuthLoginSchema = Schema.object({
  email: z
    .string()
    .email("Enter a valid email")
    .describe(
      'Email of the account to authenticate. Example: "user@example.com".',
    ),
  password: z
    .string()
    .min(1, "Password is required")
    .describe(
      "Plain text password to authenticate against the stored hash. Example: \"s3cure-pass\".",
    ),
});

/**
 * Domain SSOT for login — body-only action.
 *
 * @example
 * ```typescript
 * { email: "user@example.com", password: "my-secret-password" }
 * ```
 */
export const AuthLoginInputSchema = AuthLoginSchema;

// ============================================================================
// Status
// Response shape for authentication state probes
// ============================================================================

/**
 * Authentication status returned by `GET /auth/status`.
 *
 * @example
 * ```typescript
 * {
 *   enabled: true,
 *   hasPassword: true,
 *   isAuthenticated: true,
 *   onboarding: "done"
 * }
 * ```
 */
export const AuthStatusSchema = Schema.object({
  enabled: z
    .boolean()
    .describe("Whether authentication is required. Example: true."),
  hasPassword: z
    .boolean()
    .describe("Whether a password has been set. Example: true."),
  isAuthenticated: z
    .boolean()
    .describe(
      "Whether the current request is authenticated. Example: true.",
    ),
  onboarding: z
    .enum(["done", "waiting"])
    .describe(
      "Onboarding status: done when at least one workspace exists, waiting otherwise. Example: \"done\".",
    ),
});

// ============================================================================
// Change password
// Input for rotating the caller's password
// ============================================================================

/**
 * JSON body for changing the authenticated account password.
 *
 * Requires the current password for verification and a confirmation field to
 * prevent typos on the new password.
 *
 * @example
 * ```typescript
 * {
 *   currentPassword: "old-pass",
 *   newPassword: "new-pass",
 *   verifyPassword: "new-pass"
 * }
 * ```
 */
export const AuthChangePasswordSchema = Schema.object({
  currentPassword: z
    .string()
    .min(1, "Current password is required")
    .describe(
      "Current password for verification. Example: \"old-pass\".",
    ),
  newPassword: z
    .string()
    .min(6, "New password must be at least 6 characters")
    .describe("New password to set. Example: \"new-pass\"."),
  verifyPassword: z
    .string()
    .min(6, "Password confirmation is required")
    .describe("Must match the new password. Example: \"new-pass\"."),
}).refine((data) => data.newPassword === data.verifyPassword, {
  message: "New password and confirmation do not match",
  path: ["verifyPassword"],
});

/**
 * Domain SSOT for password change — body-only action.
 *
 * @example
 * ```typescript
 * {
 *   currentPassword: "old-pass",
 *   newPassword: "new-pass",
 *   verifyPassword: "new-pass"
 * }
 * ```
 */
export const AuthChangePasswordInputSchema = AuthChangePasswordSchema;

// ============================================================================
// Onboarding
// Input for the first-run bootstrap flow
// ============================================================================

/**
 * Nested user identity collected during onboarding.
 */
const AuthOnboardingUserSchema = Schema.object({
  name: z
    .string()
    .min(1, "Name is required")
    .describe('Display name. Example: "Felipe Barcelos".'),
  email: z
    .string()
    .email("Enter a valid email")
    .describe('Login email. Example: "user@example.com".'),
});

/**
 * Security block collected during onboarding.
 */
const AuthOnboardingSecuritySchema = Schema.object({
  password: z
    .string()
    .min(6, "Password must be at least 6 characters")
    .describe("Plain-text password for the bootstrap super account. Example: \"s3cure-pass\"."),
});

/**
 * Region preferences collected during onboarding.
 */
const AuthOnboardingRegionSchema = Schema.object({
  language: z
    .string()
    .min(1, "Language is required")
    .describe('Interface language code. Example: "pt-BR".'),
  city: z
    .string()
    .optional()
    .describe('City for locale defaults. Example: "São Paulo".'),
  country: z
    .string()
    .optional()
    .describe('Country for locale defaults. Example: "Brazil".'),
  timezone: z
    .string()
    .optional()
    .describe('IANA timezone. Example: "America/Sao_Paulo".'),
});

/**
 * Orchestrator personality collected during onboarding.
 */
const AuthOnboardingOrchestratorSchema = Schema.object({
  name: z
    .string()
    .min(1, "Give your copilot a name")
    .describe('Copilot display name. Example: "Atlas".'),
  tone: z
    .enum(["efficient", "friendly", "professional", "candid"])
    .describe('Conversational tone. Example: "friendly".'),
  style: z
    .enum(["concise", "balanced", "detailed"])
    .describe('Response length preference. Example: "balanced".'),
  autonomy: z
    .number()
    .min(0)
    .max(1)
    .describe("Autonomy level from 0 (cautious) to 1 (autonomous). Example: 0.5."),
});

/**
 * JSON body for the first-run onboarding flow.
 *
 * Creates the bootstrap super account, first workspace, and global preferences.
 *
 * @example
 * ```typescript
 * {
 *   user: { name: "Felipe", email: "user@example.com" },
 *   security: { password: "s3cure-pass" },
 *   region: { language: "pt-BR" },
 *   orchestrator: { name: "Atlas", tone: "friendly", style: "balanced", autonomy: 0.5 }
 * }
 * ```
 */
export const AuthOnboardingSchema = Schema.object({
  user: AuthOnboardingUserSchema,
  security: AuthOnboardingSecuritySchema,
  region: AuthOnboardingRegionSchema,
  orchestrator: AuthOnboardingOrchestratorSchema,
});

/**
 * Domain SSOT for onboarding — body-only action.
 *
 * @example
 * ```typescript
 * {
 *   user: { name: "Felipe", email: "user@example.com" },
 *   security: { password: "s3cure-pass" },
 *   region: { language: "pt-BR" },
 *   orchestrator: { name: "Atlas", tone: "friendly", style: "balanced", autonomy: 0.5 }
 * }
 * ```
 */
export const AuthOnboardingInputSchema = AuthOnboardingSchema;

