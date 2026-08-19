import { AosStore } from "@/app/builders/store";
import type { FractalThemeSettings } from "@/features/theme/interfaces/theme.interfaces";
import { FractalThemeSettingsSchema } from "@/features/theme/schemas/theme.schema";
import { FractalDefaultTheme } from "@/features/theme/presentation/const/default-theme";
import { FractalConfigAppearance } from "@/features/config/interfaces/config.interfaces";
import type { ThemeRadius } from "@/components/ui/radius-selector";
import { DeepMergeHelper } from "@/core/helpers/deep-merge.helper";
import { api } from "@/lib/aos-facade";
import {
  fromStoredFont,
  toStoredFont,
} from "@/features/theme/presentation/helpers/font-settings.helper";

export type ThemeMode = "light" | "dark" | "system";

/**
 * Converts one palette out of `themes_get`'s response
 * (`internal/domain/theme/entity.go`'s `Palette`, nested under
 * `theme.variants.{light,dark}`) into the shape `FractalThemeSettings`
 * expects, for `setPreset`/`update` below.
 *
 * task-12 round 3: two differences from a plain pass-through, ruled on
 * explicitly rather than guessed —
 *
 * - `semantic` → `semanticColors`: same data, Go just names the key
 *   differently. A straight rename, nothing lost.
 * - `fonts`: Go's `Palette` has no such field at all — a theme preset
 *   carries colours and a corner radius, never a font choice. Left
 *   **absent** here on purpose, not defaulted to `{}` or invented from
 *   somewhere else: `fromStoredFont(undefined)` (this feature's own
 *   helper, `presentation/helpers/font-settings.helper.ts`) already
 *   renders that as the explicit "system default" sentinel, which reads
 *   the same whether nothing was ever set or a preset switch just
 *   supplied no font — this file has no way to tell those apart, so it
 *   doesn't pretend to. Before this comment existed, the one caller that
 *   assigns this object as `theme.settings` directly (`update`'s
 *   `updatedSettings` branch, below) silently wiped out whatever font was
 *   previously chosen on every full preset switch — the "honestly empty
 *   vs actually unavailable" confusion flagged elsewhere in this port.
 * `radius`/`windows` need no truncation guard beyond `Palette`'s existing
 * `,omitempty`: an absent one here is an absent one there too.
 */
function paletteFromApi(
  palette: (Record<string, unknown> & { semantic?: Record<string, string> }) | undefined,
): Partial<FractalThemeSettings> {
  if (!palette) return {};
  const { semantic, ...rest } = palette;
  return {
    ...rest,
    semanticColors: semantic ?? {},
  } as Partial<FractalThemeSettings>;
}

export type ThemeIconSet = "minimal" | "standard" | "complete" | "none";

export type ThemeIconsConfig = {
  set: ThemeIconSet;
  colored: boolean;
};

type FractalConfigAppearanceWithRadius = FractalConfigAppearance & {
  radius?: ThemeRadius
  icons?: ThemeIconsConfig
}

export type ThemeUpdateParams = {
  mode?: ThemeMode,
  preset?: string,
  accent?: string,
  surface?: string,
  ink?: string,
  contrast?: number,
  windows?: 'solid' | 'blur',
  uiFont?: string,
  codeFont?: string,
  uiFontSize?: number,
  codeFontSize?: number,
  radius?: ThemeRadius
  icons?: Partial<ThemeIconsConfig>
}

const DEFAULT_ICONS: ThemeIconsConfig = {
  set: "complete",
  colored: true,
};

export const ThemeStore = AosStore.create("theme")
  .withState<FractalConfigAppearanceWithRadius>({
    mode: "system",
    radius: undefined,
    theme: {
      preset: "fractal",
      settings: FractalDefaultTheme.theme,
    },
    fontSizes: {
      ui: 13,
      code: 12
    },
    icons: DEFAULT_ICONS,
  })
  .addAction('setMode', (ctx) => (mode: ThemeMode) => {
    ctx.state.set({ mode })
  })
  .addAction('setPreset', (ctx) => async (preset: string) => {
    const response = await api.theme.get.query({ params: { theme: preset } });
    // task-12 round 3: `themes_get` nests palettes under
    // `theme.variants.{light,dark}` — `.theme.theme` was reading past the
    // real data on every call. See `paletteFromApi`'s own comment above
    // for what does and doesn't carry over from Go's `Palette`.
    const variants = (response.data as { theme?: { variants?: Record<string, Record<string, unknown>> } } | undefined)?.theme?.variants;
    if (variants) {
      ctx.state.set({
        theme: {
          preset,
          settings: {
            light: paletteFromApi(variants.light),
            dark: paletteFromApi(variants.dark),
          },
        },
      });
    }
  })
  .addAction('setSettings', (ctx) => (settings: Record<string, FractalThemeSettings>) => ctx.state.set({ theme: { settings } }))
  .addAction('setFontSizes', (ctx) => (fontSizes: { ui: number; code: number }) => ctx.state.set({ fontSizes }))
  .addAction('setIcons', (ctx) => (icons: Partial<ThemeIconsConfig>) => {
    const current = ctx.state.get().icons ?? DEFAULT_ICONS;
    ctx.state.set({
      icons: {
        set: icons.set ?? current.set,
        colored: icons.colored ?? current.colored,
      },
    });
  })
  .addAction('get', (ctx) => () => {
    const s = ctx.state.get();
    const activeMode = s.mode === 'system' ? window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light' : s.mode
    const settings = s.theme.settings[activeMode] || {};
    const icons = s.icons ?? DEFAULT_ICONS;

    return {
      mode: s.mode,
      activeMode: activeMode,
      preset: s.theme.preset,
      accent: settings.accent || "",
      surface: settings.surface || "",
      ink: settings.ink || "",
      contrast: settings.contrast ?? 50,
      windows: (settings.windows || "solid") as "solid" | "blur",
      uiFont: fromStoredFont(settings.fonts?.ui),
      codeFont: fromStoredFont(settings.fonts?.code),
      radius: (s.radius && s.radius !== "none" ? s.radius : settings.radius) ?? "lg",
      uiFontSize: s.fontSizes.ui,
      codeFontSize: s.fontSizes.code,
      icons,
    };
  })
  .addAction('update', (ctx) => async (values: ThemeUpdateParams): Promise<any> => {
    // #endregion
    const s = ctx.state.get();
    const newMode = values.mode ?? s.mode;
    const newPreset = values.preset ?? s.theme.preset;

    const isPresetChanged = values.preset && values.preset !== s.theme.preset;
    const isModeChanged = values.mode && values.mode !== s.mode;

    let themeBaseSettings: Record<string, any> = {};

    // 1. Se o preset ou o modo mudou, carregamos os dados base do tema da API
    if (isPresetChanged || isModeChanged) {
      const { data } = await api.theme.get.query({ params: { theme: newPreset } });
      // task-12 round 3: same `.theme.variants` fix as `setPreset` above —
      // `paletteFromApi` is what keeps `themeBaseSettings.light`/`.dark`
      // shaped the way `FractalThemeSettingsSchema` (and the merge below)
      // expects, `fonts` included-but-absent rather than silently missing.
      const variants = (data as { theme?: { variants?: Record<string, Record<string, unknown>> } } | undefined)?.theme?.variants;
      if (variants) {
        themeBaseSettings = {
          light: paletteFromApi(variants.light),
          dark: paletteFromApi(variants.dark),
        };
      }
    }

    // 2. Resolvemos o modo ativo
    const activeMode = newMode === 'system'
      ? (typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : newMode;

    // 3. Mapeamos os valores do usuário para o formato do schema (ignorando undefined)
    const shouldApplyVisualOverrides = !isModeChanged && !isPresetChanged;
    const userSettings = {
      accent: shouldApplyVisualOverrides ? values.accent : undefined,
      surface: shouldApplyVisualOverrides ? values.surface : undefined,
      ink: shouldApplyVisualOverrides ? values.ink : undefined,
      contrast: shouldApplyVisualOverrides ? values.contrast : undefined,
      windows: shouldApplyVisualOverrides ? values.windows : undefined,
      radius: values.radius,
      fonts: {
        ui: shouldApplyVisualOverrides ? toStoredFont(values.uiFont) : undefined,
        code: shouldApplyVisualOverrides ? toStoredFont(values.codeFont) : undefined,
      },
    };

    // 4. DeepMerge: último argumento tem maior prioridade (DeepMergeHelper).
    // - Preset mudou: default → salvo → user (form) → settings do tema (API) — API sobrescreve o que o user mandou.
    // - Mesmo preset: default → salvo → API (só se houve fetch p/ modo) → user — user vence.
    const defaultMode = FractalDefaultTheme.theme[activeMode as 'light' | 'dark'] || {};
    const savedMode = !isPresetChanged ? (s.theme.settings[activeMode] || {}) : {};
    const apiMode = themeBaseSettings[activeMode] || {};

    const mergedActiveModeSettings = isPresetChanged
      ? DeepMergeHelper.merge(
        FractalThemeSettingsSchema,
        defaultMode,
        apiMode
      )
      : DeepMergeHelper.merge(
        FractalThemeSettingsSchema,
        defaultMode,
        savedMode,
        apiMode,
        userSettings
      );

    // On a full preset switch this assigns `themeBaseSettings` straight
    // through, bypassing the merge below entirely — `paletteFromApi`
    // (top of file) is what makes that safe: `fonts` stays absent rather
    // than merged from the previous preset or fabricated, so a preset
    // switch is honest about supplying no font opinion instead of quietly
    // resetting one that was set.
    const updatedSettings = isPresetChanged && themeBaseSettings.dark && themeBaseSettings.light
      ? themeBaseSettings
      : {
        ...s.theme.settings,
        [activeMode]: mergedActiveModeSettings
      };

    const newRadius = isPresetChanged ? (apiMode.radius ?? mergedActiveModeSettings.radius ?? "lg") : (values.radius ?? mergedActiveModeSettings.radius ?? s.radius ?? "lg");

    const currentIcons = s.icons ?? DEFAULT_ICONS;
    const nextIcons: ThemeIconsConfig = values.icons
      ? {
          set: values.icons.set ?? currentIcons.set,
          colored: values.icons.colored ?? currentIcons.colored,
        }
      : currentIcons;

    ctx.state.set({
      mode: newMode,
      theme: {
        preset: newPreset,
        settings: updatedSettings
      },
      radius: newRadius,
      fontSizes: {
        ui: values.uiFontSize ?? s.fontSizes.ui,
        code: values.codeFontSize ?? s.fontSizes.code
      },
      icons: nextIcons,
    });

    // 5. Retornamos o estado final (já tratado) para o formulário resetar com os dados corretos
    const finalState = ctx.state.get();
    const finalActiveMode = finalState.mode === 'system'
      ? (typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
      : finalState.mode;
    const finalSettings = finalState.theme.settings[finalActiveMode] || {};
    const finalIcons = finalState.icons ?? DEFAULT_ICONS;

    return {
      mode: finalState.mode,
      activeMode: finalActiveMode,
      preset: finalState.theme.preset,
      accent: finalSettings.accent || "",
      surface: finalSettings.surface || "",
      ink: finalSettings.ink || "",
      contrast: finalSettings.contrast ?? 50,
      windows: (finalSettings.windows || "solid") as "solid" | "blur",
      uiFont: fromStoredFont(finalSettings.fonts?.ui),
      codeFont: fromStoredFont(finalSettings.fonts?.code),
      radius: (finalState.radius && finalState.radius !== "none" ? finalState.radius : finalSettings.radius) ?? "lg",
      uiFontSize: finalState.fontSizes.ui,
      codeFontSize: finalState.fontSizes.code,
      icons: finalIcons,
    };
  })
  .withPersistence({ enabled: true, storage: "localstorage" })
  .withBroadcast({ enabled: true })
  .build();
