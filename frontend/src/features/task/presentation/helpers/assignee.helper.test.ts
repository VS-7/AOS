import { describe, expect, it } from "vitest";
import { resolveTaskAssignee, type AssigneeDirectoryInput } from "./assignee.helper";

const self = { id: "9b541535-c982-4894-87f3-b6246ed65a76", name: "Vitor", username: "vitor", email: "vitor@local.test" };
const directory = {
  users: [{ id: "u-2", name: "Ana Lima", username: "ana", email: "ana@local.test" }],
  agents: [],
  self,
} as unknown as AssigneeDirectoryInput;

describe("resolveTaskAssignee", () => {
  it("keeps the daemon's projection when it resolved the assignee", () => {
    const got = resolveTaskAssignee(directory, {
      assigned: "api-builder",
      assignee: { id: "api-builder", type: "agent", name: "API Builder", role: "Feature Engineer" },
    });
    expect(got).toMatchObject({ type: "agent", name: "API Builder" });
  });

  // The daemon resolves agents only, so a task assigned to a person came back
  // as {type: "unknown"} with no name, and the sidebar printed the raw UUID.
  it("falls back to the directory when the daemon could not name the assignee", () => {
    const got = resolveTaskAssignee(directory, {
      assigned: self.id,
      assignee: { id: self.id, type: "unknown" },
    });
    expect(got).toMatchObject({ type: "user", name: "Vitor" });

    const member = resolveTaskAssignee(directory, { assigned: "u-2", assignee: { id: "u-2", type: "unknown" } });
    expect(member).toMatchObject({ type: "user", name: "Ana Lima" });
  });

  // An unassigned task answered {id: "", type: "unknown"}, which rendered a
  // "?" avatar next to "Unassigned".
  it("says nobody when the task is unassigned", () => {
    expect(resolveTaskAssignee(directory, { assigned: undefined, assignee: { id: "", type: "unknown" } })).toBeNull();
    expect(resolveTaskAssignee(directory, { assigned: "", assignee: { id: "", type: "unknown" } })).toBeNull();
  });
});
