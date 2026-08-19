import React, { useEffect, useMemo } from 'react';
import { aos } from '@/app/aos';
import { hexToOklch } from '@/lib/utils';
import type { FractalThemeSettings } from '@/features/theme/interfaces/theme.interfaces';
import type { ThemeRadius } from '@/components/ui/radius-selector';

const radiusMap: Record<ThemeRadius, string> = {
  none: '0rem',
  sm: '0.375rem',
  md: '0.75rem',
  lg: '1rem',
};

function isFractalNative(): boolean {
  return typeof window !== 'undefined' && !!window.fractal;
}

function resolveWindowsMode(windows: FractalThemeSettings['windows'] | undefined): 'solid' | 'blur' {
  if (!isFractalNative()) return 'solid';
  return windows ?? 'blur';
}

/* ──────────────────────────────────────────────────────────────
 *  Oklch helpers — pure functions suitable for memoization
 *  ───────────────────────────────────────────────────────────── */

function parseOklch(hex: string): [number, number, number] {
  const parts = hexToOklch(hex).split(' ');
  return [parseFloat(parts[0]), parseFloat(parts[1]), parseFloat(parts[2])];
}

function buildOklch(L: number, C: number, H: number, opacity?: number): string {
  const coords = `${L.toFixed(3)} ${C.toFixed(4)} ${H.toFixed(2)}`;
  return opacity !== undefined ? `oklch(${coords} / ${opacity})` : `oklch(${coords})`;
}

function mixOklch(
  hexA: string,
  hexB: string,
  weightA: number,
  weightB: number,
  opacity?: number
): string {
  const [LA, CA, HA] = parseOklch(hexA);
  const [LB, CB, HB] = parseOklch(hexB);

  const total = weightA + weightB;
  const pA = total === 0 ? 0.5 : weightA / total;
  const pB = total === 0 ? 0.5 : weightB / total;

  const L = LA * pA + LB * pB;
  const C = CA * pA + CB * pB;

  const aRad = (HA * Math.PI) / 180;
  const bRad = (HB * Math.PI) / 180;
  const x = Math.cos(aRad) * pA + Math.cos(bRad) * pB;
  const y = Math.sin(aRad) * pA + Math.sin(bRad) * pB;
  let H = (Math.atan2(y, x) * 180) / Math.PI;
  if (H < 0) H += 360;

  return buildOklch(L, C, H, opacity);
}

/* ──────────────────────────────────────────────────────────────
 *  ThemeProvider
 *  ───────────────────────────────────────────────────────────── */

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const state = aos.stores.theme.useState();

  const activeMode = useMemo(() => {
    if (state.mode === 'system') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    return state.mode;
  }, [state.mode]);

  const settings: FractalThemeSettings | undefined = state.theme.settings[activeMode];

  /** Toggle .dark / .light on <html> and sync native Electron appearance. */
  useEffect(() => {
    const root = document.documentElement;
    root.classList.remove('light', 'dark');
    root.classList.add(activeMode);

    const windowsMode = resolveWindowsMode(settings?.windows);
    root.dataset.windows = windowsMode;

    if (window.fractal?.theme?.setAppearance) {
      window.fractal.theme.setAppearance({
        mode: state.mode,
        windows: windowsMode,
        surface: settings?.surface,
      });
    }
  }, [activeMode, state.mode, settings?.windows, settings?.surface]);

  /** Pre-compute every CSS custom property derived from the active theme. */
  const themeVars = useMemo(() => {
    if (!settings) return null;

    const { surface, ink, accent, contrast, windows, semanticColors } = settings;
    const [aL, aC, aH] = parseOklch(accent);
    const effectiveWindows = resolveWindowsMode(windows);

    // Contrast drives how "strongly" mixed layers deviate from the surface.
    // Inverted: 0 ≈ 1.5 (punchy), 50 ≈ 1.0 (neutral), 100 ≈ 0.5 (subtle/lavado).
    const contrastFactor = 1.5 - contrast / 100;

    // Background — blur mode gets a translucent oklch token (Electron only)
    const [sL, sC, sH] = parseOklch(surface);
    const bgColor = effectiveWindows === 'blur'
      ? buildOklch(sL, sC, sH, 0.75)
      : buildOklch(sL, sC, sH);
    const sidebarColor = effectiveWindows === 'blur'
      ? 'transparent'
      : bgColor;

    // Visual hierarchy layers
    const mutedFill = mixOklch(surface, ink, 96 * contrastFactor, 4 / contrastFactor, 0.95);
    const cardFill = mixOklch(surface, ink, 90 * contrastFactor, 6 / contrastFactor, 0.95);
    const secondaryFill = mixOklch(surface, accent, 86 * contrastFactor, 8 / contrastFactor, 0.95);
    const popoverFill = mixOklch(surface, ink, 99 * contrastFactor, 0, 1);
    const accentFill = mixOklch(surface, ink, 94 * contrastFactor, 6 / contrastFactor);

    const vars: Record<string, string> = {
      /* Base tokens */
      '--background': bgColor,
      '--foreground': buildOklch(...parseOklch(ink)),
      '--primary': buildOklch(...parseOklch(accent)),
      '--primary-foreground': activeMode === 'dark' ? 'oklch(0 0 0)' : 'oklch(1 0 0)',
      '--radius': radiusMap[((state.radius && state.radius !== 'none' ? state.radius : settings.radius) ?? 'lg') as ThemeRadius],

      /* Surfaces */
      '--muted': mutedFill,
      '--muted-foreground': buildOklch(...parseOklch(ink), 0.55),
      '--accent': accentFill,
      '--accent-foreground': buildOklch(...parseOklch(ink)),
      '--popover': popoverFill,
      '--popover-foreground': buildOklch(...parseOklch(ink)),
      '--card': cardFill,
      '--card-foreground': buildOklch(...parseOklch(ink)),

      /* Borders & inputs */
      '--border': buildOklch(...parseOklch(ink), 0.08),
      '--input': buildOklch(...parseOklch(ink), 0.12),
      '--ring': buildOklch(...parseOklch(accent)),

      /* Secondary */
      '--secondary': secondaryFill,
      '--secondary-foreground': buildOklch(...parseOklch(ink)),

      /* Destructive */
      '--destructive': 'oklch(0.6 0.2 25)',
      '--destructive-foreground': 'oklch(0.98 0.01 25)',

      /* Charts derived from accent hue */
      '--chart-1': buildOklch(aL, aC, aH),
      '--chart-2': buildOklch(aL, aC, (aH + 160) % 360),
      '--chart-3': buildOklch(aL, aC, (aH + 30) % 360),
      '--chart-4': buildOklch(aL, aC, (aH + 200) % 360),
      '--chart-5': buildOklch(aL, aC, (aH + 90) % 360),

      /* Chat markdown (app.css) */
      '--font-chat-code-family': settings.fonts?.code
        ? `"${settings.fonts.code}", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`
        : 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      '--app-font-size-chat-code': `${state.fontSizes.code || 12}px`,
      '--color-token-text-link-foreground': buildOklch(...parseOklch(accent)),
      '--color-token-text-link-active-foreground': buildOklch(
        Math.max(aL - 0.08, 0),
        aC,
        aH,
      ),

      /* Sidebar */
      '--sidebar': sidebarColor,
      '--sidebar-foreground': buildOklch(...parseOklch(ink)),
      '--sidebar-primary': buildOklch(...parseOklch(accent)),
      '--sidebar-primary-foreground': activeMode === 'dark' ? 'oklch(0 0 0)' : 'oklch(1 0 0)',
      '--sidebar-border': buildOklch(...parseOklch(ink), 0.1),
      '--sidebar-accent': accentFill,
      '--sidebar-accent-foreground': buildOklch(...parseOklch(ink)),
      '--sidebar-ring': buildOklch(...parseOklch(accent)),
    };

    /* Semantic colours defined by the theme author */
    Object.entries(semanticColors).forEach(([key, value]) => {
      vars[`--semantic-${key}`] = buildOklch(...parseOklch(value));
    });

    return vars;
  }, [settings, state.radius, state.fontSizes.code, activeMode]);

  /** Flush every computed var to the :root element. */
  useEffect(() => {
    if (!themeVars) return;
    const root = document.documentElement;
    Object.entries(themeVars).forEach(([key, value]) => {
      root.style.setProperty(key, value);
    });
  }, [themeVars]);

  /** Dynamic Google-Fonts loading (ui + code). */
  useEffect(() => {
    const ui = settings?.fonts?.ui;
    const code = settings?.fonts?.code;
    if (!ui && !code) return;

    const fontLink = document.createElement('link');
    const families: string[] = [];
    if (ui) families.push(`family=${ui.replace(/ /g, '+')}`);
    if (code) families.push(`family=${code.replace(/ /g, '+')}`);

    if (families.length > 0) {
      fontLink.href = `https://fonts.googleapis.com/css2?${families.join('&')}&display=swap`;
      fontLink.rel = 'stylesheet';
      document.head.appendChild(fontLink);
    }

    return () => {
      if (document.head.contains(fontLink)) {
        document.head.removeChild(fontLink);
      }
    };
  }, [settings?.fonts?.ui, settings?.fonts?.code]);

  /** Font-size adjustments applied imperatively to avoid extra renders. */
  useEffect(() => {
    document.body.style.fontSize = `${state.fontSizes.ui || 13}px`;
  }, [state.fontSizes.ui]);

  return (
    <>
      {/* Smooth colour transition layer — scoped to theme-relevant properties */}
      <style>{`
        :root {
          transition: background-color 0.25s ease,
                      color 0.2s ease,
                      border-color 0.2s ease,
                      fill 0.2s ease,
                      stroke 0.2s ease;
        }
        body {
          font-family: ${settings?.fonts?.ui ? `"${settings.fonts.ui}", system-ui, sans-serif` : 'var(--font-sans)'};
        }
        code, pre {
          font-family: ${settings?.fonts?.code ? `"${settings.fonts.code}", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace` : 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace'};
          font-size: ${state.fontSizes.code || 12}px;
        }
      `}</style>
      {children}
    </>
  );
}
