import { z } from "zod";

/**
 * FractalThemeSettingsSchema: Schema for the actual color/font settings of a theme.
 * @description Defines the structured styling variables (dark/light) applied to the Fractal UI.
 */
export const FractalThemeSettingsSchema = z.object({
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
 * FractalThemeSchema: Primary schema for a theme file (*.theme.json).
 * @description Represents a Fractal UI Theme.
 */
export const FractalThemeSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  author: z.object({
    name: z.string(),
    description: z.string(),
    url: z.string()
  }),
  theme: z.object({
    dark: FractalThemeSettingsSchema,
    light: FractalThemeSettingsSchema
  })
});

/**
 * FractalTheme: TypeScript type inferred from FractalThemeSchema.
 */
export type FractalTheme = z.infer<typeof FractalThemeSchema>;

/**
 * FractalThemeSettings: TypeScript type inferred from FractalThemeSettingsSchema.
 */
export type FractalThemeSettings = z.infer<typeof FractalThemeSettingsSchema>;

/**
 * @interface IFractalThemeService
 * @description Defines the contract for the FractalThemeService managing the workspace themes.
 */
export interface IFractalThemeService {
  /**
   * List all available themes from the built-in registry and the user's ~/.fractal/themes directory.
   * @returns An array of themes.
   */
  list(): Promise<FractalTheme[]>;

  /**
   * Retrieve a theme by its id.
   * @param id - The id of the theme to retrieve.
   * @returns The theme object or null if not found.
   */
  get(id: string): Promise<FractalTheme | null>;

  /**
   * Install a new theme by copying it to ~/.fractal/themes and setting it as the active preset.
   * @param pathOrUrl - A local absolute file path or a URL to the *.theme.json file.
   * @returns The installed theme object.
   */
  install(pathOrUrl: string): Promise<FractalTheme>;
}
