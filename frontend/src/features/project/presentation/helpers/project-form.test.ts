import { describe, expect, it } from "vitest";
import { projectCreateBody, projectIdPreview, projectUpdateBody, isAbsoluteSource, hasSluggableCharacter } from "./project-form";

// The preview has to say what the daemon will actually name the project:
// internal/core/slug.Generate, whose examples these are.
describe("projectIdPreview", () => {
  it("derives the id the way the daemon does", () => {
    expect(projectIdPreview("Hello World!")).toBe("hello-world");
    expect(projectIdPreview("Café au Lait")).toBe("cafe-au-lait");
    expect(projectIdPreview("Ação Rápida Área")).toBe("acao-rapida-area");
    expect(projectIdPreview("a - b")).toBe("a-b");
    expect(projectIdPreview("snake_case name")).toBe("snake_case-name");
  });

  it("falls back to a placeholder when the name derives nothing", () => {
    expect(projectIdPreview("  --  ")).toBe("project-id");
    expect(projectIdPreview("!!!")).toBe("project-id");
  });
});

describe("the name and source rules the daemon enforces", () => {
  it("wants at least one letter or digit in a name", () => {
    expect(hasSluggableCharacter("!!!")).toBe(false);
    expect(hasSluggableCharacter("Área 51")).toBe(true);
  });

  it("wants an absolute source", () => {
    expect(isAbsoluteSource("relative/path")).toBe(false);
    expect(isAbsoluteSource("/Users/me/project")).toBe(true);
    expect(isAbsoluteSource("C:\\\\work")).toBe(true);
  });
});

describe("projectUpdateBody", () => {
  // Emptying Description and Source and saving sent only {id, name, icon},
  // so a project could never be unbound from its source.
  it("sends a cleared field as an empty string, which Go reads as clear", () => {
    expect(projectUpdateBody({ name: " Atlas ", icon: "", description: "", content: " ", source: "" })).toEqual({
      name: "Atlas",
      icon: "",
      description: "",
      content: "",
      source: "",
    });
  });
});

describe("project status", () => {
  it("is sent when the form carries one", () => {
    expect(projectUpdateBody({ name: "Atlas", status: "paused" })).toMatchObject({ status: "paused" });
    expect(projectCreateBody({ name: "Atlas", status: "done" })).toEqual({ name: "Atlas", status: "done" });
  });
});

describe("projectCreateBody", () => {
  it("leaves out what was never filled in", () => {
    expect(projectCreateBody({ name: "Atlas", icon: "", description: "", content: "", source: "" })).toEqual({
      name: "Atlas",
    });
  });
});
