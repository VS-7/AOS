import { z } from "zod";
import { Schema } from "@/core/helpers/schema.helper";

// ============================================================================
// Nested Objects
// Extracted object schemas composed into the theme entity
// ============================================================================

/**
 * Color and typography settings applied to one appearance mode (dark or light).
 *
 * Defines accent/surface/ink colors, contrast, window chrome, radius, fonts, and
 * semantic color tokens used by the AOS UI shell.
 *
 * @example
 * ```typescript
 * {
 *   accent: "#ffffff",
 *   contrast: 90,
 *   ink: "#f5f5f5",
 *   radius: "lg",
 *   windows: "blur",
 *   surface: "#0d0b08",
 *   fonts: { code: null, ui: "Inter" },
 *   semanticColors: { diffAdded: "#4ade80", diffRemoved: "#e7000b" }
 * }
 * ```
 */
export const ThemeSettingsSchema = Schema.object({
  accent: z
    .string()
    .describe('Primary accent color hex. Example: "#ffffff".'),
  contrast: z
    .number()
    .describe("Contrast level for the appearance mode. Example: 90."),
  ink: z
    .string()
    .describe('Primary text/ink color hex. Example: "#f5f5f5".'),
  radius: z
    .enum(["none", "sm", "md", "lg"])
    .optional()
    .describe('Corner radius token. Example: "lg".'),
  windows: z
    .enum(["solid", "blur"])
    .describe('Window chrome style. Example: "blur".'),
  surface: z
    .string()
    .describe('Surface/background color hex. Example: "#0d0b08".'),
  fonts: Schema.object({
    code: z
      .string()
      .nullable()
      .optional()
      .default(null)
      .describe('Monospace font family, or null for default. Example: "JetBrains Mono".'),
    ui: z
      .string()
      .nullable()
      .optional()
      .default(null)
      .describe('UI font family, or null for default. Example: "Inter".'),
  }),
  semanticColors: z
    .record(z.string(), z.string())
    .describe(
      'Named semantic color tokens (diff, skill, …). Example: { "diffAdded": "#4ade80" }.',
    ),
});

/**
 * Author metadata embedded in a theme file.
 *
 * @example
 * ```typescript
 * {
 *   name: "AOS",
 *   description: "AOS Team",
 *   url: "https://example.com"
 * }
 * ```
 */
export const ThemeAuthorSchema = Schema.object({
  name: z
    .string()
    .describe('Author display name. Example: "AOS".'),
  description: z
    .string()
    .describe('Short author bio. Example: "AOS Team".'),
  url: z
    .string()
    .describe('Author or project URL. Example: "https://example.com".'),
});

// ============================================================================
// Entity
// Full theme shape — master blueprint for *.theme.json files
// ============================================================================

/**
 * Complete UI theme entity — the master shape for built-in and user themes.
 *
 * Fields: `id` (slug), `name`, `description`, `author` ({@link ThemeAuthorSchema}),
 * and `theme.dark` / `theme.light` ({@link ThemeSettingsSchema}).
 * Action DTOs MUST derive from this export when they overlap entity fields.
 *
 * @example
 * ```typescript
 * {
 *   id: "aos",
 *   name: "AOS",
 *   description: "The default AOS OS theme",
 *   author: { name: "AOS", description: "AOS Team", url: "https://example.com" },
 *   theme: {
 *     dark: {
 *       accent: "#ffffff",
 *       contrast: 90,
 *       ink: "#f5f5f5",
 *       radius: "lg",
 *       windows: "blur",
 *       surface: "#0d0b08",
 *       fonts: { code: null, ui: "Inter" },
 *       semanticColors: { diffAdded: "#4ade80" }
 *     },
 *     light: {
 *       accent: "#000000",
 *       contrast: 90,
 *       ink: "#0d0b08",
 *       radius: "lg",
 *       windows: "blur",
 *       surface: "#f5f5f5",
 *       fonts: { code: null, ui: "Inter" },
 *       semanticColors: { diffAdded: "#16a34a" }
 *     }
 *   }
 * }
 * ```
 */
export const ThemeSchema = Schema.object({
  id: z
    .string()
    .describe('Stable theme slug used in paths and presets. Example: "aos".'),
  name: z
    .string()
    .describe('Human-readable theme name. Example: "AOS".'),
  description: z
    .string()
    .describe(
      'Short description of the theme look and feel. Example: "The default AOS OS theme".',
    ),
  author: ThemeAuthorSchema,
  theme: Schema.object({
    dark: ThemeSettingsSchema,
    light: ThemeSettingsSchema,
  }),
});

// ============================================================================
// List
// Input — no filters today (empty object SSOT for procedure/service/CLI)
// ============================================================================

/**
 * List-themes input — currently no filters (empty object).
 *
 * Bind as controller query when HTTP query === this shape.
 *
 * @example
 * ```typescript
 * {}
 * ```
 */
export const ThemeListInputSchema = Schema.object({});

// ============================================================================
// Get
// Path-only Input — domain identifier `theme`, never bare `id`
// ============================================================================

/**
 * Get-theme input — assembled from route params (`/:theme`).
 *
 * @example
 * ```typescript
 * {
 *   theme: "aos"
 * }
 * ```
 */
export const ThemeGetInputSchema = Schema.object({
  theme: z
    .string()
    .min(1)
    .describe('Theme slug from the route path. Example: "aos".'),
});

// ============================================================================
// Install
// Body-only Input — path or URL to a *.theme.json source
// ============================================================================

/**
 * Install-theme input — local absolute path or HTTPS URL to a theme file.
 *
 * When HTTP body === this shape, bind `body: ThemeInstallInputSchema`.
 *
 * @example
 * ```typescript
 * {
 *   pathOrUrl: "/Users/me/Downloads/my-theme.theme.json"
 * }
 * ```
 */
export const ThemeInstallInputSchema = Schema.object({
  pathOrUrl: z
    .string()
    .min(1)
    .describe(
      'Absolute local path or HTTPS URL to a *.theme.json file. Example: "/Users/me/Downloads/my-theme.theme.json".',
    ),
});
