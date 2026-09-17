import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { deadlineDay, dayToDeadline, formatDeadline, isDeadlineOverdue } from "./goal-deadline";

/** The value an agent writes for a local time: the wall clock and this zone's offset. */
function writtenLocally(date: Date): string {
  const offset = -date.getTimezoneOffset();
  const sign = offset < 0 ? "-" : "+";
  const pad = (n: number) => String(Math.trunc(Math.abs(n))).padStart(2, "0");
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}` +
    `${sign}${pad(offset / 60)}:${pad(offset % 60)}`
  );
}

const dayOf = (day: Date | null) => day && [day.getFullYear(), day.getMonth() + 1, day.getDate()];

// Each case runs in these zones, whatever the machine's own is. The cases
// went wrong west of UTC (the person's America/Sao_Paulo), and the first
// fix still went wrong at UTC-6 (America/Mexico_City, and America/Denver in
// summer), where 18:00 local is exactly midnight UTC.
const ZONES = [
  "UTC",
  "America/Sao_Paulo",
  "America/Mexico_City",
  "America/Denver",
  "Pacific/Pago_Pago",
  "Asia/Tokyo",
  "Pacific/Kiritimati",
];

describe.each(ZONES)("a goal deadline in %s", (zone) => {
  const machineZone = process.env.TZ;
  beforeAll(() => {
    process.env.TZ = zone;
  });
  afterAll(() => {
    // Deleted, not assigned: a machine with no TZ set (the usual case in CI)
    // has `machineZone` undefined, and `process.env.TZ = undefined` writes
    // the *string* "undefined" — a zone nothing resolves — over the rest of
    // the worker's run.
    if (machineZone === undefined) delete process.env.TZ;
    else process.env.TZ = machineZone;
  });

  describe("is a calendar day", () => {
    it("reads a date picked in the window as the day that was picked", () => {
      // The picker stores the chosen day as midnight UTC. Read in local time,
      // anywhere west of UTC that is the evening before: the list said Sep 19
      // for a deadline of Sep 20.
      expect(dayOf(deadlineDay("2026-09-20T00:00:00Z"))).toEqual([2026, 9, 20]);
      expect(dayOf(deadlineDay("2026-09-20T00:00:00.000Z"))).toEqual([2026, 9, 20]);
      expect(dayOf(deadlineDay("2026-09-20"))).toEqual([2026, 9, 20]);
    });

    it("reads an instant an agent set with a time of day in local time", () => {
      expect(dayOf(deadlineDay(writtenLocally(new Date(2026, 7, 31, 18, 0, 0))))).toEqual([2026, 8, 31]);
    });

    // The two are told apart by how the value is written, not by the instant
    // it names: 18:00 at UTC-6 and midnight UTC are the same instant, and
    // reading every midnight-UTC instant as a picked day moved that agent's
    // deadline to the next day. Only a value written at midnight with a zero
    // offset is a picked day; an agent writing a local time writes its offset.
    it("does not take a time of day written with its offset for a picked day", () => {
      const agent = "2026-08-31T18:00:00-06:00";
      const instant = new Date(agent);
      expect(dayOf(deadlineDay(agent))).toEqual([instant.getFullYear(), instant.getMonth() + 1, instant.getDate()]);
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

    it("writes an agent's local time of day as the day it falls on here", () => {
      expect(formatDeadline(writtenLocally(new Date(2026, 7, 31, 18, 0, 0)), "en")).toBe("Aug 31, 2026");
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

    it("is overdue once an instant with a time of day has passed, and not before", () => {
      const due = writtenLocally(new Date(2026, 7, 31, 18, 0));
      vi.setSystemTime(new Date(2026, 7, 31, 17, 59));
      expect(isDeadlineOverdue(due)).toBe(false);
      vi.setSystemTime(new Date(2026, 7, 31, 18, 1));
      expect(isDeadlineOverdue(due)).toBe(true);
    });
  });
});

// Each zone above restores what it found when it is done, and the zones run
// one after another — so this runs after those restores, in whatever TZ they
// left behind. On a machine with no TZ set, assigning `undefined` back wrote
// the string "undefined", which resolves to no zone at all and would follow
// every later file in this worker.
describe("restoring the machine's zone", () => {
  it("does not leave a zone nothing resolves", () => {
    expect(process.env.TZ ?? "").not.toBe("undefined");
  });
});
