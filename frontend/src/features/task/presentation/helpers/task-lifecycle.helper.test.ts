import { describe, expect, it } from "vitest";
import { allowedMoves, canMoveTo, nextStep } from "./task-lifecycle.helper";

describe("the task lifecycle as the interface offers it", () => {
  it("offers only the moves the daemon publishes", () => {
    const planning = { status: "planning" as const, nextStates: ["todo", "backlog", "stopped"] as const };
    const task = { ...planning, nextStates: [...planning.nextStates] };
    expect(canMoveTo(task, "todo")).toBe(true);
    expect(canMoveTo(task, "in_progress")).toBe(false);
    expect(canMoveTo(task, "finished")).toBe(false);
    expect(canMoveTo(task, "planning")).toBe(false);
  });

  it("offers nothing from finished, because the daemon publishes nothing", () => {
    expect(allowedMoves({ status: "finished", nextStates: [] })).toEqual([]);
  });

  it("falls back to every other status when the daemon did not say", () => {
    const moves = allowedMoves({ status: "todo" });
    expect(moves).toHaveLength(7);
    expect(moves).not.toContain("todo");
  });

  // The Start button on a planning task was refused every time, and showed
  // "the daemon is not answering" while wave 1's transport fix was missing.
  it("never offers Start where in_progress is not a move", () => {
    for (const status of ["suggestion", "backlog", "planning"] as const) {
      const step = nextStep({ status, nextStates: status === "suggestion" ? ["backlog", "finished"] : ["todo", "backlog", "stopped"] }, true);
      expect(step?.status).not.toBe("in_progress");
    }
    expect(nextStep({ status: "planning", nextStates: ["todo", "backlog", "stopped"] }, true)).toEqual({ kind: "todo", status: "todo" });
    expect(nextStep({ status: "suggestion", nextStates: ["backlog", "finished"] }, true)).toEqual({ kind: "backlog", status: "backlog" });
    expect(nextStep({ status: "todo", nextStates: ["in_progress", "backlog", "stopped"] }, true)).toEqual({ kind: "start", status: "in_progress" });
    expect(nextStep({ status: "stopped", nextStates: ["in_progress", "todo", "backlog"] }, true)).toEqual({ kind: "resume", status: "in_progress" });
  });

  // Continue on an in_progress task re-sent the status it already had.
  it("offers nothing to a task already in progress", () => {
    expect(nextStep({ status: "in_progress", nextStates: ["in_review", "stopped", "todo"] }, true)).toBeNull();
  });

  it("approves a settled review and sends an unsettled one back to work", () => {
    const review = { status: "in_review" as const, nextStates: ["finished" as const, "in_progress" as const] };
    expect(nextStep(review, true)).toEqual({ kind: "approve", status: "finished" });
    expect(nextStep(review, false)).toEqual({ kind: "continue", status: "in_progress" });
  });
});
