import { client, isDesktop, system } from "./client";

/** A palette resolved to the properties the document root carries. */
export interface RenderedTheme {
  theme: string;
  appearance: "light" | "dark";
  windows: "solid" | "blur";
  tokens: Record<string, string>;
}

/** A theme as the picker lists it. */
export interface ThemeSummary {
  id: string;
  name: string;
  author?: string;
  builtin: boolean;
}

/** What the user asked for, which may be "follow the system". */
export type AppearancePreference = "light" | "dark" | "auto";

const STORAGE_KEY = "aos.theme";

interface StoredChoice {
  theme: string;
  appearance: AppearancePreference;
  tokens?: Record<string, string>;
}

/** Reads the choice the last session made. */
export function storedChoice(): StoredChoice {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as StoredChoice;
  } catch {
    // A storage that cannot be read is a storage the page does without.
  }
  return { theme: "aos", appearance: "auto" };
}

/** What the operating system currently prefers. */
export function systemAppearance(): "light" | "dark" {
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/** Resolves a preference to the appearance it means right now. */
export function resolveAppearance(preference: AppearancePreference): "light" | "dark" {
  return preference === "auto" ? systemAppearance() : preference;
}

/**
 * Writes a rendered theme onto the document root, and tells the native layer
 * which material the window should wear.
 *
 * The second half is the desktop equivalent of the original's
 * theme:set-appearance channel: switching theme in the interface changes the
 * material of the operating system's window, not only the colours inside it.
 */
export async function applyTheme(
  rendered: RenderedTheme,
  preference: AppearancePreference,
): Promise<void> {
  const root = document.documentElement;
  for (const [token, value] of Object.entries(rendered.tokens)) {
    root.style.setProperty(`--${token}`, value);
  }
  root.classList.remove("light", "dark");
  root.classList.add(rendered.appearance);
  root.dataset["windows"] = rendered.windows;

  try {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ theme: rendered.theme, appearance: preference, tokens: rendered.tokens }),
    );
  } catch {
    // Persisting the choice is a convenience; failing to is not worth an error.
  }

  if (isDesktop()) {
    try {
      await system.setAppearance(preference, rendered.windows);
    } catch {
      // The window still repaints from the tokens set above; the native
      // material just won't match until the next successful sync.
    }
  }
}

/** Loads a theme from the daemon and applies it. */
export async function selectTheme(
  id: string,
  preference: AppearancePreference,
): Promise<RenderedTheme> {
  const out = (await client.invoke("themes_get", {
    id,
    native: isDesktop(),
    _reasoning: "the person chose a theme",
  })) as { rendered: RenderedTheme[] };

  const wanted = resolveAppearance(preference);
  const rendered =
    out.rendered.find((r) => r.appearance === wanted) ?? out.rendered[0];
  if (!rendered) throw new Error(`the theme ${id} rendered no palette`);

  await applyTheme(rendered, preference);
  return rendered;
}

/** Lists what the picker can offer. */
export async function listThemes(): Promise<ThemeSummary[]> {
  const out = (await client.invoke("themes_list", {
    _reasoning: "the theme picker is open",
  })) as { themes: ThemeSummary[] };
  return out.themes;
}

/**
 * Watches the operating system's preference and re-applies when it changes.
 *
 * Only when the preference is auto: a person who chose dark did not ask to be
 * switched to light at sunrise.
 */
export function watchSystemAppearance(onChange: () => void): () => void {
  const query = window.matchMedia("(prefers-color-scheme: dark)");
  const handler = () => onChange();
  query.addEventListener("change", handler);
  return () => query.removeEventListener("change", handler);
}
