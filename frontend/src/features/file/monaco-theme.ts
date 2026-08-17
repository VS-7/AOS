import type { editor as MonacoEditor } from "monaco-editor";

/**
 * Matches the monospace stack already used for code in
 * components/ui/markdown-content.tsx, so a code block and the Monaco editor
 * read as the same typeface.
 */
const MONOSPACE_STACK =
  'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace';

const DEFAULT_FONT_SIZE = 13;

/**
 * Resolves any CSS colour string to `#rrggbb` or `#rrggbbaa` through the DOM
 * itself, rather than hand-rolled colour math.
 *
 * Monaco's standalone themes parse every `colors` entry with
 * `Color.fromHex()`, which only accepts hex — and AOS's rendered theme
 * tokens (unlike the original's) are OKLCH CSS strings, not hex. Setting the
 * colour on a probe element and reading `getComputedStyle` back gives the
 * browser's own resolved `rgb()`/`rgba()`, which converts losslessly to hex
 * regardless of what colour syntax the token pipeline used — oklch() today,
 * anything else tomorrow.
 */
function cssColorToHex(css: string | undefined, fallback: string): string {
  if (!css || typeof document === "undefined") return fallback;
  const probe = document.createElement("div");
  probe.style.color = css;
  document.body.appendChild(probe);
  const resolved = getComputedStyle(probe).color;
  document.body.removeChild(probe);

  const match = resolved.match(
    /rgba?\((\d+(?:\.\d+)?)[,\s]+(\d+(?:\.\d+)?)[,\s]+(\d+(?:\.\d+)?)(?:[,\s/]+([\d.]+))?\)/,
  );
  if (!match) return fallback;
  const [, r, g, b, a] = match;
  const channel = (n: string) => Math.round(Number(n)).toString(16).padStart(2, "0");
  const hex = `#${channel(r ?? "0")}${channel(g ?? "0")}${channel(b ?? "0")}`;
  return a === undefined ? hex : hex + Math.round(Number(a) * 255).toString(16).padStart(2, "0");
}

function withAlpha(hex: string, alpha: number): string {
  const base = hex.length >= 7 ? hex.slice(0, 7) : hex;
  return base + Math.round(Math.max(0, Math.min(1, alpha)) * 255).toString(16).padStart(2, "0");
}

/**
 * Reads a rendered theme token straight off the document root's CSS custom
 * properties, rather than through a value threaded down from ThemePicker.
 *
 * theme.ts's applyTheme sets every one of Render's tokens as `--<name>` on
 * document.documentElement the moment a theme is chosen, so the root is
 * already the single live source of "what theme is active right now" — a
 * second channel here would just be a second place that value can go stale.
 */
function readToken(name: string): string | undefined {
  if (typeof document === "undefined") return undefined;
  const value = getComputedStyle(document.documentElement).getPropertyValue(`--${name}`).trim();
  return value || undefined;
}

export interface MonacoThemePayload {
  name: string;
  fontFamily: string;
  fontSize: number;
  theme: MonacoEditor.IStandaloneThemeData;
}

/**
 * Builds a Monaco standalone theme from whatever theme is currently applied
 * to the document. This is not a port of the original's theme builder —
 * AOS's theme system is a different, smaller model (four base colours and a
 * contrast dial, deriving everything else, rather than the original's
 * ~40-token per-theme table) — so the mapping below picks the closest AOS
 * token for each Monaco colour rather than reproducing a token list that no
 * longer exists on this side.
 */
export function buildMonacoTheme(): MonacoThemePayload {
  const dark = document.documentElement.classList.contains("dark");

  const background = cssColorToHex(readToken("background"), dark ? "#111111" : "#ffffff");
  const foreground = cssColorToHex(readToken("foreground"), dark ? "#fcfcfc" : "#0d0d0d");
  const primary = cssColorToHex(readToken("primary"), dark ? "#93c5fd" : "#1d4ed8");
  const muted = cssColorToHex(readToken("muted-foreground"), dark ? "#8a8a8a" : "#6b6b6b");
  const border = cssColorToHex(readToken("border"), dark ? "#2a2a2a" : "#e5e5e5");
  const card = cssColorToHex(readToken("card"), background);
  const diffAdded = cssColorToHex(readToken("semantic-diffAdded"), "#00a240");
  const diffRemoved = cssColorToHex(readToken("semantic-diffRemoved"), "#e02e2a");
  const chartString = cssColorToHex(readToken("chart-2"), primary);
  const chartNumber = cssColorToHex(readToken("chart-3"), primary);

  return {
    name: `aos-monaco-${dark ? "dark" : "light"}`,
    fontFamily: MONOSPACE_STACK,
    fontSize: DEFAULT_FONT_SIZE,
    theme: {
      base: dark ? "vs-dark" : "vs",
      inherit: true,
      rules: [
        { token: "comment", foreground: muted.slice(1) },
        { token: "keyword", foreground: primary.slice(1), fontStyle: "bold" },
        { token: "string", foreground: chartString.slice(1) },
        { token: "number", foreground: chartNumber.slice(1) },
        { token: "regexp", foreground: chartString.slice(1) },
        { token: "type", foreground: primary.slice(1) },
      ],
      colors: {
        "editor.background": background,
        "editor.foreground": foreground,
        "editor.lineHighlightBackground": card,
        "editorLineNumber.foreground": muted,
        "editorLineNumber.activeForeground": foreground,
        "editorGutter.background": background,
        "editorCursor.foreground": primary,
        "editor.selectionBackground": withAlpha(primary, dark ? 0.28 : 0.2),
        "editor.inactiveSelectionBackground": withAlpha(primary, dark ? 0.14 : 0.1),
        "editor.wordHighlightBackground": withAlpha(primary, dark ? 0.12 : 0.08),
        "editorIndentGuide.background1": withAlpha(foreground, 0.08),
        "editorIndentGuide.activeBackground1": withAlpha(foreground, 0.16),
        "editorWidget.background": card,
        "editorWidget.border": border,
        "editorOverviewRuler.border": border,
        "diffEditor.insertedTextBackground": withAlpha(diffAdded, 0.2),
        "diffEditor.removedTextBackground": withAlpha(diffRemoved, 0.2),
        "scrollbarSlider.background": withAlpha(foreground, dark ? 0.16 : 0.1),
        "scrollbarSlider.hoverBackground": withAlpha(foreground, dark ? 0.22 : 0.14),
        "scrollbarSlider.activeBackground": withAlpha(foreground, dark ? 0.28 : 0.18),
      },
    },
  };
}
