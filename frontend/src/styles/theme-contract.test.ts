import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// node:fs, not a `?raw` import: Vitest stubs every CSS import to an empty
// string (its `css: false` default), `?raw` included, so the bundler cannot
// hand this suite the stylesheet's own text. `process.cwd()` because the
// jsdom environment the rest of the suite runs under gives `import.meta.url`
// an http:// value; Vitest always runs from the frontend root.
const frontendDir = process.cwd();
const srcDir = join(frontendDir, "src");

const appCss = readFileSync(join(srcDir, "styles", "app.css"), "utf8");
const indexHtml = readFileSync(join(frontendDir, "index.html"), "utf8");

/** Every source file under src/, so a sweep cannot miss a new one. */
function sourceFiles(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) sourceFiles(path, out);
    else if (/\.(ts|tsx|css)$/.test(entry)) out.push(path);
  }
  return out;
}

/** Files reading a token the wrong way — this suite's own text excluded. */
function filesContaining(needle: string): string[] {
  return sourceFiles(srcDir)
    .filter((path) => !path.endsWith("theme-contract.test.ts"))
    .filter((path) => readFileSync(path, "utf8").includes(needle))
    .map((path) => path.slice(frontendDir.length + 1))
    .sort();
}

/**
 * The theme is not a component, so nothing else in this suite would catch it
 * breaking. Each case here corresponds to a fidelity bug that shipped and
 * looked, from a screenshot, like "the theme is a bit off": a stray line, a
 * corner that didn't match, a colour that silently didn't apply. The
 * assertions are on the stylesheet and the pre-paint script as text, because
 * that is where each of those bugs actually lived.
 */

describe("the global stylesheet", () => {
  it("gives every element a border colour from the token set", () => {
    // Tailwind v4's preflight resets borders to `border: 0 solid` — no colour,
    // which CSS resolves to `currentColor`. Without this rule the ~380 bare
    // `border` / `border-t` / `divide-*` classes across the ported components
    // draw their line in the *text* colour: a bright white line on every dark
    // theme, appearing in places nothing asked for a border.
    expect(appCss).toMatch(/border-color:\s*var\(--border\)/);
  });

  it("keeps every global rule inside a cascade layer", () => {
    // Unlayered declarations outrank every layered one no matter the
    // specificity, so a bare `button { … }` here silently beats `bg-primary`
    // and `rounded-md` on all of the ported components. Only the at-rules that
    // must stay at the top level are allowed outside a layer.
    const withoutLayers = appCss
      .replace(/@layer[^{]*\{[\s\S]*?\n\}/g, "")
      .replace(/@theme[^{]*\{[\s\S]*?\n\}/g, "")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/@import[^;]*;/g, "")
      .replace(/@custom-variant[^;]*;/g, "");
    expect(withoutLayers.trim()).toBe("");
  });

  it("derives the whole radius scale from --radius, in order", () => {
    // Overriding only sm/md/lg/xl left Tailwind's defaults in place for the
    // rest, which both ignored the theme's corner setting and put
    // `rounded-2xl` (1rem) *below* the overridden `rounded-xl` (--radius +
    // 4px) — the ported tree uses both.
    for (const step of ["xs", "sm", "md", "lg", "xl", "2xl", "3xl", "4xl"]) {
      expect(appCss).toMatch(new RegExp(`--radius-${step}:[^;]*var\\(--radius\\)`));
    }
  });
});

describe("the pre-paint script", () => {
  it("reads the key the theme store actually writes", () => {
    // AosStore keys as `<appPrefix>:<name>` (app/builders/store.ts), so the
    // theme store's is `aos:theme`. It read `aos.theme` — the key the
    // superseded lib/theme.ts used — so the lookup always missed and the
    // first frame painted with no appearance class at all.
    expect(indexHtml).toContain('localStorage.getItem("aos:theme")');
    expect(indexHtml).not.toContain('"aos.theme"');
  });

  it("puts an appearance class on <html> before React mounts", () => {
    // Every `dark:` rule in the ported tree keys off this class, so a first
    // frame without it renders light-mode styling over a dark surface.
    expect(indexHtml).toMatch(/classList\.add\(appearance\)/);
  });
});

describe("token references", () => {
  it("never wraps a token in hsl()", () => {
    // These tokens hold complete `oklch(...)` colours, not the bare HSL
    // triples shadcn used before v4. `hsl(oklch(…))` is invalid, so the whole
    // declaration is dropped — the error screens rendered with no background
    // at all, and several focus rings simply never drew.
    expect(filesContaining("hsl(var(--")).toEqual([]);
  });

  it("never reads --sidebar-background, which no theme produces", () => {
    // shadcn's pre-v4 name for what this token set calls `--sidebar`.
    expect(filesContaining("--sidebar-background")).toEqual([]);
  });
});
