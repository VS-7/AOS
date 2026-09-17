import { describe, expect, it } from "vitest";
import type { Run } from "@/features/routine/interfaces/routine.interfaces";
import { RoutineRunHelper } from "./routine-run.helper";

const run = (patch: Partial<Run>): Run => ({
  id: "run-1",
  routine: "r-1",
  agent: "luara",
  trigger: "manual",
  status: "succeeded",
  startedAt: "2026-09-13T09:00:00Z",
  endedAt: "2026-09-13T09:02:30Z",
  ...patch,
});

describe("RoutineRunHelper", () => {
  // Every run showed a red "Failed": the badge knew only "completed".
  it("reads Go's run statuses", () => {
    expect(RoutineRunHelper.outcome("succeeded")).toBe("succeeded");
    expect(RoutineRunHelper.outcome("failed")).toBe("failed");
    expect(RoutineRunHelper.outcome("timed_out")).toBe("failed");
    expect(RoutineRunHelper.outcome("skipped")).toBe("skipped");
    expect(RoutineRunHelper.outcome("running")).toBe("running");
  });

  // Duration read `finishedAt`, which Go calls `endedAt`.
  it("measures a run from startedAt to endedAt", () => {
    expect(RoutineRunHelper.duration(run({}))).toBe("3m");
    expect(RoutineRunHelper.duration(run({ endedAt: "2026-09-13T09:00:12Z" }))).toBe("12s");
    expect(RoutineRunHelper.duration(run({ endedAt: undefined }))).toBe("—");
  });

  it("counts outcomes within a window", () => {
    const now = new Date("2026-09-13T12:00:00Z").getTime();
    const runs = [
      run({ id: "a" }),
      run({ id: "b", status: "timed_out" }),
      run({ id: "c", status: "failed", startedAt: "2026-09-01T09:00:00Z", endedAt: "2026-09-01T09:01:00Z" }),
    ];
    expect(RoutineRunHelper.countInWindow(runs, "succeeded", 24 * 3600_000, now)).toBe(1);
    expect(RoutineRunHelper.countInWindow(runs, "failed", 24 * 3600_000, now)).toBe(1);
    expect(RoutineRunHelper.countInWindow(runs, "failed", 30 * 24 * 3600_000, now)).toBe(2);
  });
});
