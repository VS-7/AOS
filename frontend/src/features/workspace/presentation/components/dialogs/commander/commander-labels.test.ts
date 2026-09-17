import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import en from "@/lib/i18n/locales/en.json";
import ptBR from "@/lib/i18n/locales/pt-BR.json";

/**
 * The palette translates a command's label and group when it renders them,
 * through `t(command.label)` — a variable, which the catalogue test's scan for
 * `t("…")` literals cannot see. Without this, a trigger added in English
 * renders English inside the Portuguese palette and nothing says so, which is
 * exactly how every heading and command there ended up untranslated.
 */
describe("the command palette's words", () => {
  it("are all in both catalogues", () => {
    const words = new Set<string>();
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "node_modules") walk(path);
          continue;
        }
        if (!entry.name.endsWith(".trigger.ts")) continue;
        const source = readFileSync(path, "utf8");
        for (const pattern of [/\blabel: "([^"]+)"/g, /\bgroup: "([^"]+)"/g, /AosTriggerGroup\.create\("([^"]+)"\)/g]) {
          for (const match of source.matchAll(pattern)) words.add(match[1]);
        }
      }
    };
    walk("src/features");

    expect(words.size).toBeGreaterThan(20);
    const missing = [...words].filter((word) => !(word in en) || !(word in ptBR)).sort();
    expect(missing).toEqual([]);
  });
});
