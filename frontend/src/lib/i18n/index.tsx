import * as React from "react";
import en from "./locales/en.json";
import ptBR from "./locales/pt-BR.json";

/**
 * The interface, in the language the person reads.
 *
 * Two catalogues, flat JSON, one file per language — `en.json` is the source
 * of truth for what keys exist, `pt-BR.json` mirrors it. Flat and not nested
 * on purpose: a nested catalogue reads better in an editor and much worse in
 * a diff, and the thing this file is judged by is whether a missing key is
 * obvious when somebody adds a screen.
 *
 * There are two ways in, because the strings live in two kinds of place:
 *
 * - `useTranslation()` inside a component, which re-renders when the
 *   language changes.
 * - `t(...)` imported directly, for the ~300 `toast.*` calls, helper
 *   modules and constant tables that are not components and never will be.
 *   It reads the same active locale; it just cannot subscribe to it, which
 *   costs nothing because a toast is written once, at the moment it fires.
 */

export const LOCALES = ["en", "pt-BR"] as const;
export type Locale = (typeof LOCALES)[number];

/** What each language calls itself, for the picker. */
export const LOCALE_NAMES: Record<Locale, string> = {
  en: "English",
  "pt-BR": "Português (Brasil)",
};

type Catalog = Record<string, string>;

const CATALOGS: Record<Locale, Catalog> = {
  en: en as Catalog,
  "pt-BR": ptBR as Catalog,
};

const STORAGE_KEY = "aos.locale";

/**
 * Maps anything a browser, a webview or the workspace configuration might
 * report onto a catalogue we have.
 *
 * `pt`, `pt-PT` and `pt-br` all land on `pt-BR`: shipping one Portuguese and
 * refusing to show it to somebody whose system says `pt` would be a strange
 * kind of correctness.
 */
export function normalizeLocale(value: string | null | undefined): Locale | null {
  if (!value) return null;
  const tag = value.trim().toLowerCase().replace("_", "-");
  if (!tag) return null;
  if (tag === "pt" || tag.startsWith("pt-")) return "pt-BR";
  if (tag === "en" || tag.startsWith("en-")) return "en";
  return null;
}

function readStoredLocale(): Locale | null {
  try {
    return normalizeLocale(localStorage.getItem(STORAGE_KEY));
  } catch {
    // Private mode, a webview with site data off — not a reason to fail.
    return null;
  }
}

/** The language to start in, before the workspace configuration is known. */
export function detectLocale(): Locale {
  const stored = readStoredLocale();
  if (stored) return stored;
  if (typeof navigator !== "undefined") {
    for (const tag of navigator.languages ?? [navigator.language]) {
      const match = normalizeLocale(tag);
      if (match) return match;
    }
  }
  return "en";
}

// The active locale, kept in a module variable so the non-hook `t` below can
// read it. The provider is what writes it.
let activeLocale: Locale = typeof window === "undefined" ? "en" : detectLocale();
const listeners = new Set<() => void>();

/** The language the interface is currently in. */
export function getLocale(): Locale {
  return activeLocale;
}

/**
 * Switches language, remembers the choice, and re-renders everything reading
 * it through `useTranslation`.
 */
export function setLocale(next: Locale): void {
  if (next === activeLocale) return;
  activeLocale = next;
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // The choice still applies to this session.
  }
  if (typeof document !== "undefined") {
    document.documentElement.lang = next;
  }
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Values substituted into `{{placeholders}}`. */
export type TVars = Record<string, string | number | undefined | null>;

function interpolate(template: string, vars?: TVars): string {
  if (!vars) return template;
  return template.replace(/\{\{(\w+)\}\}/g, (whole, name: string) => {
    const value = vars[name];
    return value === undefined || value === null ? whole : String(value);
  });
}

/**
 * Looks a key up in the active catalogue.
 *
 * A key the active catalogue does not have falls back to English, and a key
 * neither has falls back to `fallback` — the English text the call site was
 * written with — and finally to the key itself. That order is what makes it
 * safe to add a screen in one language: it renders in English until somebody
 * translates it, rather than rendering `settings.workspace.title` at a user.
 */
export function translate(locale: Locale, key: string, vars?: TVars, fallback?: string): string {
  const hit = CATALOGS[locale]?.[key] ?? CATALOGS.en[key] ?? fallback ?? key;
  return interpolate(hit, vars);
}

/**
 * Translate outside React — toasts, helpers, constant tables.
 *
 * Same catalogue, same fallbacks; it simply reads the locale at call time
 * instead of subscribing to it.
 */
export function t(key: string, vars?: TVars, fallback?: string): string {
  return translate(activeLocale, key, vars, fallback);
}

interface I18nValue {
  locale: Locale;
  setLocale: (next: Locale) => void;
  t: (key: string, vars?: TVars, fallback?: string) => string;
}

const I18nContext = React.createContext<I18nValue | null>(null);

/**
 * Publishes the active language to the tree.
 *
 * It holds no state of its own beyond a subscription: the locale lives in the
 * module above so that `t` outside React and `t` inside it can never disagree.
 */
export function I18nProvider({ children }: { children: React.ReactNode }): React.JSX.Element {
  const locale = React.useSyncExternalStore(subscribe, getLocale, () => "en" as Locale);

  React.useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const value = React.useMemo<I18nValue>(
    () => ({
      locale,
      setLocale,
      t: (key, vars, fallback) => translate(locale, key, vars, fallback),
    }),
    [locale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

/**
 * The hook every component uses.
 *
 * It works with no provider above it — falling back to the module locale —
 * so a component rendered in a test, or in a tree mounted before the
 * provider (an error boundary's fallback, say), still reads real strings
 * rather than throwing.
 */
export function useTranslation(): I18nValue {
  const context = React.useContext(I18nContext);
  const locale = React.useSyncExternalStore(subscribe, getLocale, () => "en" as Locale);
  return (
    context ?? {
      locale,
      setLocale,
      t: (key, vars, fallback) => translate(locale, key, vars, fallback),
    }
  );
}

/**
 * Adopts the language the workspace configuration records, unless the person
 * has since picked one in this browser.
 *
 * Onboarding writes `region.language`, so this is what makes the choice made
 * there apply to the interface rather than only to what the agents are told.
 */
export function applyConfiguredLocale(language: string | null | undefined): void {
  if (readStoredLocale()) return;
  const match = normalizeLocale(language);
  if (match) setLocale(match);
}
