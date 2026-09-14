import { describe, expect, it } from "vitest";
import { countByStatus, openingStatus } from "./status-tabs";

const ORDER = ["todo", "in_progress", "stopped", "finished"] as const;

// The project's Tasks tab opened on "Todo" and said "No tasks in this
// status." while its only task sat, unannounced, behind the "Stopped" icon.
describe("status tabs", () => {
  it("count every item under its status, zero for the rest", () => {
    expect(countByStatus(ORDER, [{ status: "stopped" }, { status: "stopped" }, { status: "todo" }])).toEqual({
      todo: 1,
      in_progress: 0,
      stopped: 2,
      finished: 0,
    });
  });

  it("open on the first status that has something in it", () => {
    const counts = countByStatus(ORDER, [{ status: "stopped" }]);
    expect(openingStatus(ORDER, counts, null)).toBe("stopped");
  });

  it("open on the first status when there is nothing at all", () => {
    expect(openingStatus(ORDER, countByStatus(ORDER, []), null)).toBe("todo");
  });

  it("keep the tab the person chose, even when it is empty", () => {
    const counts = countByStatus(ORDER, [{ status: "stopped" }]);
    expect(openingStatus(ORDER, counts, "finished")).toBe("finished");
  });
});
