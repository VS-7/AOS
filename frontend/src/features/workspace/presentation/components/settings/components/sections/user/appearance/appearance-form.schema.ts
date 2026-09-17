import * as z from "zod";

/** The sizes the interface can be drawn at and still hold together. */
export const FONT_SIZE_MIN = 9;
export const FONT_SIZE_MAX = 24;

/**
 * What the appearance section may save.
 *
 * It sits beside the section rather than inside it so it can be read on its
 * own: the section itself needs a canvas, a colour picker and the theme list
 * to render, and the rule worth pinning is which values it refuses.
 */
export const appearanceFormSchema = z.object({
  mode: z.enum(["light", "dark", "system"]),
  preset: z.string(),
  accent: z.string(),
  surface: z.string(),
  ink: z.string(),
  contrast: z.number().min(0).max(100),
  windows: z.enum(["solid", "blur"]),
  uiFont: z.string().optional(),
  codeFont: z.string().optional(),
  radius: z.enum(["none", "sm", "md", "lg"]),
  // Bounded: an emptied field used to store 0 and report success, and the
  // interface quietly fell back to 13px over a setting that read 0.
  uiFontSize: z.number().min(FONT_SIZE_MIN).max(FONT_SIZE_MAX),
  codeFontSize: z.number().min(FONT_SIZE_MIN).max(FONT_SIZE_MAX),
  iconsSet: z.enum(["minimal", "standard", "complete", "none"]),
  iconsColored: z.boolean(),
});
