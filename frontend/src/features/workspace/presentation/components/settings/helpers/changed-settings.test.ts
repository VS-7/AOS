import { describe, expect, it } from "vitest";
import { changedSettings } from "./changed-settings";

describe("changedSettings", () => {
  it("sends only the fields that differ from what was saved, under their dotted path", () => {
    const saved = { branchPrefix: "feat/", forcePush: false, commitInstructions: "Conventional." };
    const next = { branchPrefix: "feat/", forcePush: true, commitInstructions: "Conventional." };

    // The autosave used to send the whole group: a field left at a value
    // read before another save overwrote that save.
    expect(changedSettings("git", next, saved)).toEqual({ "git.forcePush": true });
  });

  it("addresses top-level fields without a prefix", () => {
    expect(changedSettings("", { name: "VS", color: "#000000" }, { name: "VS", color: "#ffffff" })).toEqual({
      color: "#000000",
    });
  });

  it("says nothing changed when nothing did", () => {
    expect(changedSettings("worktrees", { worktreeLimit: 5 }, { worktreeLimit: 5 })).toEqual({});
  });

  it("compares lists and objects by value, and sends them whole", () => {
    const saved = { tasks: [{ id: "bug", label: "Bug" }] };
    expect(changedSettings("", { tasks: [{ id: "bug", label: "Bug" }] }, saved)).toEqual({});
    expect(changedSettings("", { tasks: [{ id: "bug", label: "Defect" }] }, saved)).toEqual({
      tasks: [{ id: "bug", label: "Defect" }],
    });
  });

  it("treats a field missing from the saved values as changed", () => {
    expect(changedSettings("region", { city: "Recife" }, undefined)).toEqual({ "region.city": "Recife" });
  });
});
