import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import { resolveMonacoLanguageFromFileName } from "@/features/file/presentation/helpers/file-viewer.helper";
import { DefaultTheme } from "@/features/theme/presentation/const/default-theme";
import type { ThemeSettings } from "@/features/theme/interfaces/theme.interfaces";
import type { editor as MonacoEditor } from "monaco-editor";

interface ThemeState {
  fontSizes: {
    code: number;
  };
  mode: "light" | "dark" | "system";
  theme: {
    settings: Partial<Record<"light" | "dark", ThemeSettings>>;
  };
}

interface ResolvedThemeState {
  activeMode: "light" | "dark";
  codeFont: string;
  codeFontSize: number;
  settings: ThemeSettings;
}

interface MonacoThemePayload {
  name: string;
  options: {
    fontFamily: string;
    fontSize: number;
  };
  theme: MonacoEditor.IStandaloneThemeData;
}

function normalizeHex(value: string | undefined, fallback: string) {
  if (!value) return fallback;

  const normalized = value.trim();

  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) return normalized;

  if (/^#[0-9a-fA-F]{3}$/.test(normalized)) {
    const [r, g, b] = normalized.slice(1);
    return `#${r}${r}${g}${g}${b}${b}`;
  }

  return fallback;
}

function parseHex(hex: string) {
  const normalized = normalizeHex(hex, "#000000");

  return {
    b: Number.parseInt(normalized.slice(5, 7), 16),
    g: Number.parseInt(normalized.slice(3, 5), 16),
    r: Number.parseInt(normalized.slice(1, 3), 16),
  };
}

function channelToHex(value: number) {
  return Math.round(Math.max(0, Math.min(255, value)))
    .toString(16)
    .padStart(2, "0");
}

function mixHex(from: string, to: string, weight: number) {
  const source = parseHex(from);
  const target = parseHex(to);
  const clampedWeight = Math.max(0, Math.min(1, weight));

  return `#${channelToHex(source.r + (target.r - source.r) * clampedWeight)}${channelToHex(
    source.g + (target.g - source.g) * clampedWeight,
  )}${channelToHex(source.b + (target.b - source.b) * clampedWeight)}`;
}

/**
 * Monaco standalone themes parse every `colors` entry with `Color.fromHex()`, which only
 * accepts `#RGB` / `#RRGGBB` / `#RRGGBBAA`. `rgba()` strings fail and fall back to red.
 */
function withAlpha(hex: string, alpha: number) {
  const { r, g, b } = parseHex(hex);
  const a = Math.round(Math.max(0, Math.min(1, alpha)) * 255);

  return `#${channelToHex(r)}${channelToHex(g)}${channelToHex(b)}${channelToHex(a)}`;
}

function toMonacoForeground(hex: string) {
  return normalizeHex(hex, "#000000").slice(1);
}

function resolveThemeState(state: ThemeState): ResolvedThemeState {
  const activeMode =
    state.mode === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : state.mode;

  const fallbackSettings = DefaultTheme.theme[activeMode];
  const settings = state.theme.settings[activeMode] ?? fallbackSettings;

  return {
    activeMode,
    codeFont: settings.fonts?.code || fallbackSettings.fonts.code || "BerkeleyMono",
    codeFontSize: state.fontSizes.code || 8,
    settings,
  };
}

export function buildMonacoTheme(state: ThemeState): MonacoThemePayload {
  const { activeMode, codeFont, codeFontSize, settings } = resolveThemeState(state);
  const surface = normalizeHex(settings.surface, activeMode === "dark" ? "#111111" : "#ffffff");
  const ink = normalizeHex(settings.ink, activeMode === "dark" ? "#fcfcfc" : "#0d0d0d");
  const accent = normalizeHex(settings.accent, activeMode === "dark" ? "#ffffff" : "#0d0d0d");
  const diffAdded = normalizeHex(settings.semanticColors["diffAdded"], "#00a240");
  const diffRemoved = normalizeHex(settings.semanticColors["diffRemoved"], "#e02e2a");
  const lineHighlight = mixHex(surface, ink, activeMode === "dark" ? 0.08 : 0.03);
  const gutter = mixHex(surface, ink, activeMode === "dark" ? 0.04 : 0.02);
  const border = mixHex(surface, ink, activeMode === "dark" ? 0.12 : 0.08);
  const muted = mixHex(surface, ink, activeMode === "dark" ? 0.45 : 0.35);
  /** Selection / focus: neutral blue so UI stays readable even when brand accent is red or warm. */
  const selectionTint = activeMode === "dark" ? "#3b82f6" : "#2563eb";
  const lineNumberMuted = mixHex(surface, ink, activeMode === "dark" ? 0.38 : 0.32);
  const lineNumberActive = mixHex(surface, ink, activeMode === "dark" ? 0.62 : 0.5);
  const stringColor = activeMode === "dark" ? mixHex(accent, "#ffffff", 0.24) : mixHex(accent, "#9a3412", 0.42);
  const keywordColor = activeMode === "dark" ? mixHex(accent, "#93c5fd", 0.35) : mixHex(accent, "#1d4ed8", 0.5);
  const numberColor = activeMode === "dark" ? mixHex(accent, "#f59e0b", 0.55) : mixHex(accent, "#b45309", 0.55);

  return {
    name: `aos-files-${activeMode}`,
    options: {
      fontFamily: `"${codeFont}", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`,
      fontSize: codeFontSize,
    },
    theme: {
      base: activeMode === "dark" ? "vs-dark" : "vs",
      inherit: true,
      rules: [
        { token: "comment", foreground: toMonacoForeground(muted) },
        { token: "keyword", foreground: toMonacoForeground(keywordColor), fontStyle: "bold" },
        { token: "string", foreground: toMonacoForeground(stringColor) },
        { token: "number", foreground: toMonacoForeground(numberColor) },
        { token: "regexp", foreground: toMonacoForeground(accent) },
        { token: "type.identifier", foreground: toMonacoForeground(mixHex(accent, ink, 0.3)) },
      ],
      colors: {
        "diffEditor.insertedTextBackground": withAlpha(diffAdded, 0.2),
        "diffEditor.removedTextBackground": withAlpha(diffRemoved, 0.2),
        "editor.background": surface,
        "editor.findMatchBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.32 : 0.22),
        "editor.findMatchHighlightBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.18 : 0.12),
        "editor.foreground": ink,
        "editor.inactiveSelectionBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.14 : 0.1),
        "editor.lineHighlightBackground": lineHighlight,
        "editor.selectionBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.28 : 0.2),
        "editor.selectionHighlightBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.16 : 0.12),
        "editor.wordHighlightBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.12 : 0.08),
        "editor.wordHighlightStrongBackground": withAlpha(selectionTint, activeMode === "dark" ? 0.18 : 0.12),
        "editorCursor.foreground": mixHex(ink, accent, activeMode === "dark" ? 0.35 : 0.25),
        "editorIndentGuide.activeBackground1": mixHex(surface, ink, activeMode === "dark" ? 0.2 : 0.14),
        "editorIndentGuide.background1": mixHex(surface, ink, activeMode === "dark" ? 0.1 : 0.06),
        "editorGutter.background": surface,
        "editorLineNumber.activeForeground": lineNumberActive,
        "editorLineNumber.dimmedForeground": lineNumberMuted,
        "editorLineNumber.foreground": lineNumberMuted,
        "editorOverviewRuler.border": border,
        "editorWhitespace.foreground": withAlpha(ink, activeMode === "dark" ? 0.12 : 0.08),
        "editorWidget.background": mixHex(surface, ink, activeMode === "dark" ? 0.08 : 0.03),
        "editorWidget.border": border,
        "input.background": mixHex(surface, ink, activeMode === "dark" ? 0.05 : 0.02),
        "input.border": border,
        "list.activeSelectionBackground": withAlpha(accent, activeMode === "dark" ? 0.18 : 0.12),
        "list.hoverBackground": withAlpha(ink, activeMode === "dark" ? 0.06 : 0.04),
        "minimap.selectionHighlight": withAlpha(selectionTint, activeMode === "dark" ? 0.35 : 0.25),
        "minimapSlider.activeBackground": withAlpha(ink, activeMode === "dark" ? 0.2 : 0.12),
        "minimapSlider.background": withAlpha(ink, activeMode === "dark" ? 0.12 : 0.08),
        "panel.border": border,
        "peekView.border": border,
        "peekViewEditor.background": surface,
        "peekViewResult.background": gutter,
        "scrollbar.shadow": withAlpha(surface, 0),
        "scrollbarSlider.activeBackground": withAlpha(ink, activeMode === "dark" ? 0.28 : 0.18),
        "scrollbarSlider.background": withAlpha(ink, activeMode === "dark" ? 0.16 : 0.1),
        "scrollbarSlider.hoverBackground": withAlpha(ink, activeMode === "dark" ? 0.22 : 0.14),
      },
    },
  };
}

export function resolveMonacoLanguage(file: WorkspaceFile) {
  return resolveMonacoLanguageFromFileName(file.name, file.extension);
}
