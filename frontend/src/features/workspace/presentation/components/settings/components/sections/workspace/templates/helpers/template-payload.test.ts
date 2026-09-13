import { describe, expect, it } from "vitest";
import { templateCreatePayload, templateUpdatePayload } from "./template-payload";

const PROMPT = "<rules>\n- item\n</rules>\n<!-- note -->\nHi {{ name }} \\ done\n\n";

describe("template payloads", () => {
  it("keeps the body exactly as typed", () => {
    // A template body is Liquid a model reads; trimming it changed it.
    expect(templateCreatePayload({ name: "Plan", description: "d", content: PROMPT }).content).toBe(PROMPT);
    expect(templateUpdatePayload({ name: "Plan", description: "d", content: PROMPT }).content).toBe(PROMPT);
  });

  it("sends a cleared field as empty on update, so the daemon clears it", () => {
    // Dropped keys read as "leave unchanged": the save reported success and
    // the old variables and output came back.
    const body = templateUpdatePayload({ name: "Plan", description: "", output: "  ", variablesText: "", content: "x" });
    expect(body).toMatchObject({ description: "", output: "", variables: [] });
    expect(body).not.toHaveProperty("skill");
  });

  it("leaves empty optional fields out of a create", () => {
    expect(templateCreatePayload({ name: " Plan ", description: "d", output: "", variablesText: " ", content: "x" })).toEqual({
      name: "Plan",
      description: "d",
      content: "x",
    });
  });

  it("says what is wrong with the variables", () => {
    expect(() => templateUpdatePayload({ name: "P", description: "d", variablesText: '{"name":"version"}', content: "x" })).toThrow(
      /list/,
    );
    expect(() => templateUpdatePayload({ name: "P", description: "d", variablesText: "[{", content: "x" })).toThrow(
      /Variables must be valid JSON/,
    );
  });
});
