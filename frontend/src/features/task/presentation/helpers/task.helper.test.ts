import { describe, expect, it } from "vitest";
import { TaskHelper } from "./task.helper";

describe("TaskHelper", () => {
  it("shows the first block of a task id", () => {
    expect(TaskHelper.shortId("0186fedb-0e6c-4856-bd81-ec695709f859")).toBe("0186fedb");
    expect(TaskHelper.shortId("t-42")).toBe("t");
    expect(TaskHelper.shortId("plain")).toBe("plain");
  });

  // "Copy branch name" copied `docs/harness-beta`; the daemon cuts
  // `aos/harness-beta`.
  it("names the branch the daemon cuts, not one built from the type", () => {
    const task = { id: "t1", slug: "harness-beta", worktree: { enabled: true } };
    expect(TaskHelper.branchName(task)).toBe("aos/harness-beta");
    expect(TaskHelper.branchName(task, "team")).toBe("team/harness-beta");
    expect(TaskHelper.branchName({ ...task, worktree: { enabled: true, branch: "feature/x" } }, "team")).toBe("feature/x");
  });
});
