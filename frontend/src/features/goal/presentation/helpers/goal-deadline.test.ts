import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { deadlineDay, dayToDeadline, formatDeadline, isDeadlineOverdue } from "./goal-deadline";

// These run in whatever time zone the machine has; the cases below are the
// ones that went wrong west of UTC (the person's America/Sao_Paulo), and they
// are written so they hold in any zone.
describe("a goal deadline is a calendar day", () => {
  it("reads a date picked in the window as the day that was picked", () => {
    // The picker stores the chosen day as midnight UTC. Read in local time,
    // anywhere west of UTC that is the evening before: the list said Sep 19
    // for a deadline of Sep 20.
    const day = deadlineDay("2026-09-20T00:00:00Z");
    expect(day && [day.getFullYear(), day.getMonth() + 1, day.getDate()]).toEqual([2026, 9, 20]);
  });

  it("reads an instant an agent set with a time of day in local time", () => {
    const local = new Date(2026, 7, 31, 18, 0, 0);
    const day = deadlineDay(local.toISOString());
    expect(day && [day.getFullYear(), day.getMonth() + 1, day.getDate()]).toEqual([2026, 8, 31]);
  });

  it("stores a picked day as that day's midnight UTC, whatever the zone", () => {
    expect(dayToDeadline(new Date(2026, 8, 20))).toBe("2026-09-20T00:00:00.000Z");
    expect(dayToDeadline(new Date(2026, 8, 20, 23, 59))).toBe("2026-09-20T00:00:00.000Z");
  });

  it("has no day for nothing, or for text that is not a date", () => {
    expect(deadlineDay(undefined)).toBeNull();
    expect(deadlineDay("")).toBeNull();
    expect(deadlineDay("soon")).toBeNull();
  });
});

describe("formatDeadline", () => {
  it("writes the day in the interface's language, not the browser's", () => {
    expect(formatDeadline("2026-09-20T00:00:00Z", "en")).toBe("Sep 20, 2026");
    expect(formatDeadline("2026-09-20T00:00:00Z", "pt-BR")).toBe("20 de set. de 2026");
  });

  it("writes nothing for no deadline", () => {
    expect(formatDeadline(undefined, "en")).toBeNull();
  });
});

describe("isDeadlineOverdue", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("is not overdue on the day itself", () => {
    vi.setSystemTime(new Date(2026, 8, 20, 22, 0));
    expect(isDeadlineOverdue("2026-09-20T00:00:00Z")).toBe(false);
  });

  it("is overdue from the next day on", () => {
    vi.setSystemTime(new Date(2026, 8, 21, 0, 1));
    expect(isDeadlineOverdue("2026-09-20T00:00:00Z")).toBe(true);
  });

  it("is overdue once an instant with a time of day has passed", () => {
    vi.setSystemTime(new Date(2026, 7, 31, 18, 1));
    expect(isDeadlineOverdue(new Date(2026, 7, 31, 18, 0).toISOString())).toBe(true);
  });
});
