import { describe, expect, it } from "vitest";
import { appearanceFormSchema } from "./appearance-form.schema";

const values = {
  mode: "system" as const,
  preset: "aos",
  accent: "oklch(0.7 0.1 80)",
  surface: "oklch(0.97 0 90)",
  ink: "oklch(0.15 0 80)",
  contrast: 50,
  windows: "solid" as const,
  radius: "lg" as const,
  uiFontSize: 13,
  codeFontSize: 12,
  iconsSet: "standard" as const,
  iconsColored: true,
};

// An emptied size box holds NaN and an out-of-range one holds a number the
// interface cannot be drawn at; both used to be saved — 0 px was stored and
// the window silently fell back to 13 (#203).
describe("the appearance form", () => {
  it("takes a size the interface can be drawn at", () => {
    expect(appearanceFormSchema.safeParse(values).success).toBe(true);
    expect(appearanceFormSchema.safeParse({ ...values, uiFontSize: 9, codeFontSize: 24 }).success).toBe(true);
  });

  it("refuses a size outside the range, and an emptied box", () => {
    for (const size of [0, 8, 25, Number.NaN]) {
      const result = appearanceFormSchema.safeParse({ ...values, uiFontSize: size });
      expect(result.success, `uiFontSize ${size}`).toBe(false);
      expect(appearanceFormSchema.safeParse({ ...values, codeFontSize: size }).success).toBe(false);
    }
  });
});
