import { describe, expect, it } from "vitest";
import { translate } from "@/lib/i18n";
import ptBR from "@/lib/i18n/locales/pt-BR.json";
import { themeRadiusOptions } from "./radius-selector";

describe("the corner radius options", () => {
  it("label with keys the catalogues carry", () => {
    for (const option of themeRadiusOptions) {
      expect(Object.hasOwn(ptBR, option.label)).toBe(true);
    }
  });

  // "Medium" is the priority scales' key, and pt-BR reads it as the feminine
  // "Média" — agreeing with "prioridade". Among "Nenhum", "Pequeno" and
  // "Grande" it was the one word in the wrong gender.
  it("do not borrow a label whose translation agrees with another noun", () => {
    const translated = themeRadiusOptions.map((option) => translate("pt-BR", option.label));
    expect(translated).not.toContain(translate("pt-BR", "Medium"));
  });
});
