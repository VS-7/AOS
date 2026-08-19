import { z } from "zod";

/**
 * Schema for login request validation.
 * The password is plain text and will be verified against the stored hash.
 */
export const AuthLoginSchema = z.object({
  password: z
    .string()
    .min(1, "Password is required")
    .describe("Plain text password to authenticate against the stored hash."),
});

/**
 * Schema for session/token response.
 * The token is generated upon successful authentication and must be sent
 * in subsequent requests as a Bearer token or cookie.
 */
export const AuthStatusSchema = z.object({
  enabled: z.boolean().describe("Whether authentication is required."),
  hasPassword: z.boolean().describe("Whether a password has been set."),
  isAuthenticated: z
    .boolean()
    .describe("Whether the current request is authenticated."),
  onboarding: z
    .enum(["done", "waiting"])
    .describe(
      "Onboarding status: 'done' if at least one workspace exists, 'waiting' otherwise.",
    ),
});

/**
 * Schema for changing the password.
 * Requires the current password for verification, the new password,
 * and a confirmation of the new password to prevent typos.
 */
export const AuthChangePasswordSchema = z
  .object({
    currentPassword: z
      .string()
      .min(1, "Current password is required")
      .describe("Current password for verification."),
    newPassword: z
      .string()
      .min(6, "New password must be at least 6 characters")
      .describe("New password to set."),
    verifyPassword: z
      .string()
      .min(6, "Password confirmation is required")
      .describe("Must match the new password."),
  })
  .refine((data) => data.newPassword === data.verifyPassword, {
    message: "New password and confirmation do not match",
    path: ["verifyPassword"],
  });

/**
 * Schema for API token response.
 */
export const AuthApiTokenSchema = z.object({
  token: z
    .string()
    .describe(
      "The API token value. Shown once on generation, then only the hash is stored.",
    ),
});

/**
 * Schema for API token response.
 */
export const AuthRegenerateApiTokenSchema = z.object({});

// Inferred types
export type AuthLogin = z.infer<typeof AuthLoginSchema>;
export type AuthStatus = z.infer<typeof AuthStatusSchema>;
export type AuthChangePassword = z.infer<typeof AuthChangePasswordSchema>;
export type AuthApiToken = z.infer<typeof AuthApiTokenSchema>;

/**
 * Schema for onboarding request validation.
 * Contains all data needed to bootstrap a workspace and configure Fractal.
 */
export const AuthOnboardingSchema = z.object({
  user: z.object({
    name: z.string().min(1, "Name is required"),
    email: z.string().email("Enter a valid email"),
  }),
  security: z.object({
    password: z.string().min(6, "Password must be at least 6 characters"),
  }),
  region: z.object({
    language: z.string().min(1, "Language is required"),
    city: z.string().optional(),
    country: z.string().optional(),
    timezone: z.string().optional(),
  }),
  orchestrator: z.object({
    name: z.string().min(1, "Give your copilot a name"),
    tone: z.enum(["efficient", "friendly", "professional", "candid"]),
    style: z.enum(["concise", "balanced", "detailed"]),
    autonomy: z.number().min(0).max(1),
  }),
});

export type AuthOnboarding = z.infer<typeof AuthOnboardingSchema>;

/**
 * Aliases for the freshly-copied `v401/web` presentation code, which
 * imports the `Fractal`-prefixed names — same reasoning as `agent.
 * interfaces.ts`'s `FractalAgent`.
 */
export type FractalAuthChangePassword = AuthChangePassword;
export type FractalAuthOnboarding = AuthOnboarding;
