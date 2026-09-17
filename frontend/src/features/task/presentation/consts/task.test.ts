import { describe, expect, it } from "vitest";
import { TASK_STATUS_CONFIG, TASK_PRIORITY_CONFIG } from "./task";

// The status icon is the only mark a column header or a list row carries for
// the status — the label is not repeated on every card — so two statuses
// drawn with the same glyph are two statuses a reader cannot tell apart.
// Backlog and Todo shared lucide's dashed circle (imported twice, under two
// names), the same way Stopped once shared Suggestion's.
describe("TASK_STATUS_CONFIG", () => {
  it("draws every status with its own icon", () => {
    const icons = Object.values(TASK_STATUS_CONFIG).map((config) => config.icon);
    expect(new Set(icons).size).toBe(icons.length);
  });
});

describe("TASK_PRIORITY_CONFIG", () => {
  it("draws every priority with its own icon", () => {
    const icons = Object.values(TASK_PRIORITY_CONFIG).map((config) => config.icon);
    expect(new Set(icons).size).toBe(icons.length);
  });
});
