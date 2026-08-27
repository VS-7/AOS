import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it, expect, beforeEach } from "vitest";
import en from "./locales/en.json";
import ptBR from "./locales/pt-BR.json";
import { LOCALES, LOCALE_NAMES, normalizeLocale, detectLocale, getLocale, setLocale, t, translate } from "./index";

/**
 * The one test that actually protects the feature: a key added to a screen and
 * never translated renders English inside a Portuguese interface, and nothing
 * else in the build says so.
 */
describe("the catalogues", () => {
  it("cover exactly the same keys", () => {
    const inEnglish = Object.keys(en).sort();
    const inPortuguese = Object.keys(ptBR).sort();

    const untranslated = inEnglish.filter((key) => !(key in ptBR));
    const orphaned = inPortuguese.filter((key) => !(key in en));

    expect(untranslated, "keys with no Portuguese").toEqual([]);
    expect(orphaned, "Portuguese for a string nothing renders").toEqual([]);
  });

  it("has no empty translation", () => {
    const blank = Object.entries(ptBR).filter(([, value]) => !String(value).trim());
    expect(blank).toEqual([]);
  });

  // A Portuguese value identical to the English one is usually a forgotten
  // entry rather than a word that happens to be the same. A handful genuinely
  // are — proper names, commands, format tags — so this asserts on the count
  // rather than forbidding them, which would make the catalogue unwritable.
  it("is actually translated", () => {
    const identical = Object.entries(ptBR).filter(([key, value]) => key === value);
    expect(identical.length).toBeLessThan(Object.keys(en).length * 0.12);
  });

  // The drift this catches: a screen written with `t("Something new")` and no
  // catalogue entry renders the key. In English that is invisible — the key
  // *is* the English text — so the Portuguese interface is the only place it
  // shows, and only to someone reading that screen in Portuguese. Eight keys
  // had already slipped through by the time this was written.
  it("has an entry for every key the source asks for", () => {
    const keys = new Set<string>();
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "node_modules") walk(path);
          continue;
        }
        if (!/\.(ts|tsx)$/.test(entry.name) || /\.test\.tsx?$/.test(entry.name)) continue;
        const source = readFileSync(path, "utf8");
        for (const match of source.matchAll(/(?<![\w.])t\(\s*"((?:[^"\\]|\\.)*)"/g)) {
          keys.add(JSON.parse(`"${match[1]}"`));
        }
      }
    };
    walk("src");

    expect(keys.size).toBeGreaterThan(500);
    const uncatalogued = [...keys].filter((key) => !(key in en)).sort();
    expect(uncatalogued, "t() keys with no catalogue entry").toEqual([]);
  });

  it("keeps every interpolation the English string declares", () => {
    const placeholders = (s: string) => (s.match(/\{\{\w+\}\}/g) ?? []).sort();
    for (const [key, value] of Object.entries(ptBR)) {
      expect(placeholders(String(value)), `placeholders drifted for ${key}`).toEqual(placeholders(key));
    }
  });
});

describe("locale resolution", () => {
  // This runner's jsdom leaves `localStorage` undefined unless node is started
  // with --localstorage-file, which is exactly the shape a private window or a
  // webview with site data off has — and the reason every access in index.tsx
  // sits inside a try/catch. The shim is here so these assertions test the
  // resolution logic rather than the absence of a browser API; the catalogue
  // suite above runs without it and proves the module still loads.
  const memory = new Map<string, string>();
  beforeEach(() => {
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: {
        getItem: (key: string) => memory.get(key) ?? null,
        setItem: (key: string, value: string) => void memory.set(key, value),
        removeItem: (key: string) => void memory.delete(key),
        clear: () => memory.clear(),
        key: () => null,
        length: 0,
      },
    });
    memory.clear();
    setLocale("en");
  });

  // Somebody whose system says `pt` or `pt-PT` gets the Portuguese we have,
  // rather than English on a technicality.
  it("maps every Portuguese tag onto the catalogue that exists", () => {
    for (const tag of ["pt", "pt-BR", "pt-br", "pt_BR", "pt-PT"]) {
      expect(normalizeLocale(tag), tag).toBe("pt-BR");
    }
    for (const tag of ["en", "en-US", "en-GB"]) {
      expect(normalizeLocale(tag), tag).toBe("en");
    }
    expect(normalizeLocale("fr-FR")).toBeNull();
    expect(normalizeLocale("")).toBeNull();
    expect(normalizeLocale(null)).toBeNull();
  });

  it("remembers the choice and applies it to t()", () => {
    setLocale("pt-BR");
    expect(getLocale()).toBe("pt-BR");
    expect(localStorage.getItem("aos.locale")).toBe("pt-BR");
    expect(t("Settings")).toBe("Configurações");
    expect(detectLocale()).toBe("pt-BR");
  });

  it("falls back to English, then to the key itself", () => {
    expect(translate("pt-BR", "a key nothing has")).toBe("a key nothing has");
    expect(translate("pt-BR", "a key nothing has", undefined, "Fallback")).toBe("Fallback");
  });

  it("substitutes placeholders and leaves unknown ones alone", () => {
    expect(translate("en", "Hello {{name}}", { name: "Vitor" }, "Hello {{name}}")).toBe("Hello Vitor");
    expect(translate("en", "Hello {{name}}", {}, "Hello {{name}}")).toBe("Hello {{name}}");
  });

  it("names every locale it offers", () => {
    for (const locale of LOCALES) {
      expect(LOCALE_NAMES[locale]).toBeTruthy();
    }
  });
});
