import { describe, it, expect } from "vitest";
import { WorkspaceUpdateInputSchema, WORKSPACE_NAME_REQUIRED } from "./workspace.schema";

// Clearing the Name field in Settings autosaved — the schema let an empty
// name through — and the switcher then read "No Workspace" as if none were
// selected. The form only submits what its schema accepts.
describe("WorkspaceUpdateInputSchema", () => {
  it("refuses a workspace with no name", () => {
    for (const name of ["", "   "]) {
      const parsed = WorkspaceUpdateInputSchema.safeParse({ name, logo: "", color: "#5ed296" });
      expect(parsed.success).toBe(false);
      expect(parsed.error?.issues[0]?.message).toBe(WORKSPACE_NAME_REQUIRED);
    }
  });

  it("keeps the name without the spaces around it", () => {
    const parsed = WorkspaceUpdateInputSchema.safeParse({ name: "  VS  ", logo: "", color: "#5ed296" });
    expect(parsed.success).toBe(true);
    expect(parsed.data?.name).toBe("VS");
  });

  it("still takes a patch that does not name the workspace", () => {
    expect(WorkspaceUpdateInputSchema.safeParse({ color: "#5ed296" }).success).toBe(true);
  });
});
