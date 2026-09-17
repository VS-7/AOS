import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const calls = vi.hoisted(() => ({
  update: vi.fn(),
  setStatus: vi.fn(),
  remove: vi.fn(),
  branch: vi.fn(),
  confirm: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));

vi.mock("@/app/aos", () => ({
  aos: {
    client: {
      task: {
        update: { mutateOrThrow: calls.update },
        setStatus: { mutateOrThrow: calls.setStatus },
        delete: { mutateOrThrow: calls.remove },
        branch: { mutateOrThrow: calls.branch },
      },
    },
    stores: {
      agent: { useState: (select: (s: unknown) => unknown) => select({ items: [{ id: "luara", name: "Luara" }] }) },
      workspace: {
        useState: (select: (s: unknown) => unknown) => select({ directory: { users: [{ id: "u-1", name: "Vitor" }], agents: [] } }),
        actions: { refreshDirectory: vi.fn(async () => {}) },
      },
      auth: { useState: (select: (s: unknown) => unknown) => select({ user: null }) },
    },
  },
}));
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm: calls.confirm }) }));
vi.mock("sonner", () => ({ toast: { success: calls.success, error: calls.error } }));

import { useTaskActions } from "./task-actions.hook";
import type { Task } from "@/features/task/interfaces/task.interfaces";

const task = {
  id: "7aa73525-24f3-45ba-a74d-2fa7c1ffe288",
  name: "Harness Alpha",
  slug: "harness-alpha",
  type: "bug",
  priority: "urgent",
  status: "planning",
  nextStates: ["todo", "backlog", "stopped"],
  assigned: "luara",
  project: "api-de-teste",
  goal: "api-de-biblioteca-funcional",
  dueAt: "2026-08-31T21:00:00Z",
  worktree: { enabled: false },
  attachments: [],
  assignee: { id: "luara", type: "agent", name: "Luara" },
} as unknown as Task;

function actions() {
  const onChanged = vi.fn();
  const { result } = renderHook(() => useTaskActions(task, { onChanged }));
  return { actions: result.current, onChanged };
}

beforeEach(() => {
  for (const fn of Object.values(calls)) fn.mockReset();
  calls.update.mockResolvedValue({});
  calls.setStatus.mockResolvedValue({});
  calls.remove.mockResolvedValue({});
});

describe("useTaskActions", () => {
  // `undefined` is dropped by JSON, and tasks_update reads a missing key as
  // "leave it": "No assignee" toasted "Unassigned" over a task that kept its
  // owner, and the same for the project, the goal and the due date.
  it("clears a field by sending an empty string, not by leaving the key out", async () => {
    const { actions: a, onChanged } = actions();
    await act(async () => {
      await a.setAssignee(undefined);
      await a.setProject(undefined);
      await a.setGoal(undefined);
      await a.setDueDate(undefined);
    });
    const bodies = calls.update.mock.calls.map(([opts]) => opts.body);
    expect(bodies).toEqual([{ assigned: "" }, { project: "" }, { goal: "" }, { dueAt: "" }]);
    for (const body of bodies) expect(JSON.parse(JSON.stringify(body))).toEqual(body);
    expect(onChanged).toHaveBeenCalledTimes(4);
  });

  it("names the assignee it set", async () => {
    const { actions: a } = actions();
    await act(async () => {
      await a.setAssignee("luara");
    });
    expect(calls.update).toHaveBeenCalledWith({ params: { task: task.id }, body: { assigned: "luara" } });
    expect(calls.success).toHaveBeenCalledWith("Assigned to Luara");
  });

  it("does not ask the daemon for a move the lifecycle does not have", async () => {
    const { actions: a, onChanged } = actions();
    await act(async () => {
      expect(await a.setStatus("in_progress")).toBe(false);
    });
    expect(calls.setStatus).not.toHaveBeenCalled();
    expect(calls.error).toHaveBeenCalledWith("Failed to update status", {
      description: "A task in Planning cannot move to In Progress.",
    });
    expect(onChanged).not.toHaveBeenCalled();

    await act(async () => {
      expect(await a.setStatus("todo")).toBe(true);
    });
    expect(calls.setStatus).toHaveBeenCalledWith({ params: { task: task.id }, body: { status: "todo" } });
  });

  it("shows the daemon's reason when it refuses", async () => {
    calls.update.mockRejectedValue(new Error('"2026-09-30" is not an RFC3339 instant'));
    const { actions: a, onChanged } = actions();
    await act(async () => {
      expect(await a.setDueDate("2026-09-30")).toBe(false);
    });
    expect(calls.error).toHaveBeenCalledWith("Failed to update due date", {
      description: '"2026-09-30" is not an RFC3339 instant',
    });
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Delete used to remove the task, its plan, its discussion and its runs on
  // one click.
  it("deletes only after the person confirms, and names the task it deleted", async () => {
    const { actions: a } = actions();
    calls.confirm.mockResolvedValue(false);
    await act(async () => {
      expect(await a.remove()).toBe(false);
    });
    expect(calls.remove).not.toHaveBeenCalled();
    expect(calls.confirm.mock.calls[0][0].description).toContain("Harness Alpha");

    calls.confirm.mockResolvedValue(true);
    await act(async () => {
      expect(await a.remove()).toBe(true);
    });
    expect(calls.remove).toHaveBeenCalledWith({ params: { task: task.id } });
    expect(calls.success).toHaveBeenLastCalledWith("Task deleted: Harness Alpha");
  });
});
