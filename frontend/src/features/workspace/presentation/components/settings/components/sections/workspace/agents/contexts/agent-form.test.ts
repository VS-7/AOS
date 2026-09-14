import { describe, expect, it } from "vitest";
import { agentSlug, buildAgentFormValues, buildCreatePayload, buildUpdatePayload } from "./agent-form";

describe("agentSlug", () => {
  // The cases internal/core/slug's own tests pin, so the form's "this name
  // makes no id" warning agrees with the daemon that will refuse it.
  it.each([
    ["Hello World!", "hello-world"],
    ["Café au Lait", "cafe-au-lait"],
    ["  --  ", ""],
    ["!!!", ""],
    ["Luára", "luara"],
    ["a - b", "a-b"],
    ["API Builder", "api-builder"],
    ["snake_case 2", "snake_case-2"],
    ["日本", "日本"],
  ])("%j becomes %j", (name, id) => {
    expect(agentSlug(name)).toBe(id);
  });
});

describe("buildUpdatePayload", () => {
  it("sends nothing for a field nobody touched", () => {
    const values = buildAgentFormValues({ id: "a", name: "A", role: "Old", orchestrator: false });
    expect(buildUpdatePayload(values, {})).toEqual({});
  });

  it("clears with an empty string rather than dropping the key", () => {
    const values = { ...buildAgentFormValues({ id: "a", name: "A", orchestrator: false }), role: "   " };
    expect(buildUpdatePayload(values, { role: true })).toEqual({ role: "" });
  });

  it("never sends a skill", () => {
    const values = { ...buildAgentFormValues(null), skill: "backend" } as never;
    expect(buildUpdatePayload(values, { skill: true } as never)).toEqual({});
  });
});

describe("buildCreatePayload", () => {
  it("leaves blanks to the daemon's defaults and keeps the instructions verbatim", () => {
    const values = {
      ...buildAgentFormValues(null),
      name: " Revisor ",
      content: "<rules>\n- a  \n</rules>\n",
    };
    expect(buildCreatePayload(values)).toEqual({
      name: "Revisor",
      content: "<rules>\n- a  \n</rules>\n",
      orchestrator: false,
    });
  });
});
