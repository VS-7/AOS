import { describe, expect, it } from "vitest";
import { instructionCreatePayload, instructionUpdatePayload } from "./instruction-payload";

const PROMPT = "<rules>\n- item\n</rules>\n<!-- note -->\nUse {{ var }} and \\ literally.\n";

describe("instruction payloads", () => {
  it("keeps the body exactly as typed", () => {
    expect(instructionCreatePayload({ name: "N", type: "standards", content: PROMPT }).content).toBe(PROMPT);
    expect(instructionUpdatePayload({ name: "N", type: "standards", content: PROMPT }).content).toBe(PROMPT);
  });

  it("sends a cleared description and paths on update, so the instruction becomes global", () => {
    expect(instructionUpdatePayload({ name: "N", type: "standards", description: " ", pathsText: "\n", content: "x" })).toMatchObject({
      description: "",
      paths: [],
    });
  });

  it("reads one glob per line and leaves empty fields out of a create", () => {
    expect(instructionCreatePayload({ name: " N ", type: "standards", pathsText: "a/**\n\n b/*.ts ", content: "" })).toEqual({
      name: "N",
      type: "standards",
      paths: ["a/**", "b/*.ts"],
    });
  });
});
