import { cn } from "@/lib/utils";

export const THEME_PREVIEW_SWATCH_SIZE = 20;

export type ThemePreviewColors = {
  accent: string;
  surface: string;
  ink: string;
};

type ThemePreviewSwatchProps = {
  colors: ThemePreviewColors;
  className?: string;
  size?: number;
};

/**
 * Circular swatch previewing accent, background, and foreground colors.
 */
export function ThemePreviewSwatch({
  colors,
  className,
  size = THEME_PREVIEW_SWATCH_SIZE,
}: ThemePreviewSwatchProps) {
  return (
    <span
      aria-hidden
      className={cn("inline-block shrink-0 rounded-md", className)}
      style={{
        width: size,
        height: size,
        background: `conic-gradient(from 180deg, ${colors.ink} 0deg 180deg, ${colors.accent} 180deg 270deg, ${colors.surface} 270deg 360deg)`,
      }}
    />
  );
}

export function getThemePreviewColors(
  theme: { theme?: { light: ThemePreviewColors; dark: ThemePreviewColors } },
  mode: "light" | "dark",
): ThemePreviewColors | null {
  const settings = theme.theme?.[mode] ?? theme.theme?.light;
  if (!settings) return null;

  return {
    accent: settings.accent ?? "#888888",
    surface: settings.surface ?? "#ffffff",
    ink: settings.ink ?? "#000000",
  };
}
