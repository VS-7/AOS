/**
 * A day kept as an instant.
 *
 * A date picker with no time of day stores the chosen day as that day's
 * midnight UTC. Read back in local time, that instant is the evening before
 * anywhere west of UTC, so a deadline picked for the 20th showed as the 19th.
 * A value somebody set with a real time of day (an agent writing
 * 2026-08-31T18:00:00-03:00) means the day it falls on where the person is.
 *
 * So midnight UTC exactly reads as a picked day, in UTC, and anything else as
 * an instant, in local time.
 */

function parse(value?: string | null): Date | null {
  if (!value?.trim()) return null;
  const instant = new Date(value);
  return Number.isNaN(instant.getTime()) ? null : instant;
}

/** Whether an instant is what a date-only picker writes: midnight UTC. */
export function isPickedDay(instant: Date): boolean {
  return (
    instant.getUTCHours() === 0 &&
    instant.getUTCMinutes() === 0 &&
    instant.getUTCSeconds() === 0 &&
    instant.getUTCMilliseconds() === 0
  );
}

/** The instant a value names, or null for nothing or for text that is not one. */
export function instantOf(value?: string | null): Date | null {
  return parse(value);
}

/** The calendar day a value falls on, as a local-midnight Date. */
export function calendarDayOf(value?: string | null): Date | null {
  const instant = parse(value);
  if (!instant) return null;
  return isPickedDay(instant)
    ? new Date(instant.getUTCFullYear(), instant.getUTCMonth(), instant.getUTCDate())
    : new Date(instant.getFullYear(), instant.getMonth(), instant.getDate());
}

/** What a date-only picker stores for a chosen day: its midnight UTC. */
export function dayAsUtcMidnight(day: Date): string {
  return new Date(Date.UTC(day.getFullYear(), day.getMonth(), day.getDate())).toISOString();
}
