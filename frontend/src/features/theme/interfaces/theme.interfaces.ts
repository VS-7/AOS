import { z } from "zod";

/**
 * ThemeSettingsSchema: Schema for the actual color/font settings of a theme.
 * @description Defines the structured styling variables (dark/light) applied to the AOS UI.
 */
export const ThemeSettingsSchema = z.object({
  accent: z.string(),
  contrast: z.number(),
  ink: z.string(),
  radius: z.enum(["none", "sm", "md", "lg"]).optional(),
  windows: z.enum(['solid', 'blur']),
  surface: z.string(),
  fonts: z.object({
    code: z.string().optional().nullable().default("IoskeleyMono"),
    ui: z.string().optional().nullable().default("Inter")
  }),
  semanticColors: z.record(z.string(), z.string())
});

/**
 * ThemeSchema: Primary schema for a theme file (*.theme.json).
 * @description Represents a AOS UI Theme.
 */
export const ThemeSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  author: z.object({
    name: z.string(),
    description: z.string(),
    url: z.string()
  }),
  theme: z.object({
    dark: ThemeSettingsSchema,
    light: ThemeSettingsSchema
  })
});

/**
 * Theme: TypeScript type inferred from ThemeSchema.
 */
export type Theme = z.infer<typeof ThemeSchema>;

/**
 * ThemeSettings: TypeScript type inferred from ThemeSettingsSchema.
 */
export type ThemeSettings = z.infer<typeof ThemeSettingsSchema>;

/**
 * @interface IThemeService
 * @description Defines the contract for the ThemeService managing the workspace themes.
 */
export interface IThemeService {
  /**
   * List all available themes from the built-in registry and the user's ~/.aos/themes directory.
   * @returns An array of themes.
   */
  list(): Promise<Theme[]>;

  /**
   * Retrieve a theme by its id.
   * @param id - The id of the theme to retrieve.
   * @returns The theme object or null if not found.
   */
  get(id: string): Promise<Theme | null>;

  /**
   * Install a new theme by copying it to ~/.aos/themes and setting it as the active preset.
   * @param pathOrUrl - A local absolute file path or a URL to the *.theme.json file.
   * @returns The installed theme object.
   */
  install(pathOrUrl: string): Promise<Theme>;
}
