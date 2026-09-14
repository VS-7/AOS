/**
 * A day kept as an instant.
 *
 * A date picker with no time of day stores the chosen day as that day's
 * midnight UTC. Read back in local time, that instant is the evening before
 * anywhere west of UTC, so a deadline picked for the 20th showed as the 19th.
 * A value somebody set with a real time of day (an agent writing
 * 2026-08-31T18:00:00-03:00) means the day it falls on where the person is.
 *
 * The two are told apart by how the value is *written*, not by the instant
 * it names. Midnight UTC is also 18:00 at UTC-6: judged by the instant,
 * an agent's 18:00 deadline in Mexico City read as a picked day, one day
 * late, and became overdue a day late. So only a value written as midnight
 * with a zero offset (`…T00:00:00Z`, `…T00:00:00+00:00`), or as a bare date,
 * is a picked day, read in UTC; anything else is an instant, read in local
 * time. The daemon keeps the offset a deadline was written with
 * (internal/domain/goal parseDueAt), and a local time written as UTC
 * (`Date.prototype.toISOString`) at exactly midnight is the one value this
 * cannot tell from a picked day.
 */

function parse(value?: string | null): Date | null {
  if (!value?.trim()) return null;
  const instant = new Date(value);
  return Number.isNaN(instant.getTime()) ? null : instant;
}

/** A bare date, or midnight written with a zero offset. */
const PICKED_DAY = /^\d{4}-\d{2}-\d{2}(?:T00:00(?::00(?:\.0+)?)?(?:Z|[+-]00:?00))?$/i;

/** Whether a value is written the way a date-only picker writes a day. */
export function isPickedDay(value?: string | null): boolean {
  return !!value && PICKED_DAY.test(value.trim()) && parse(value) !== null;
}

/** The instant a value names, or null for nothing or for text that is not one. */
export function instantOf(value?: string | null): Date | null {
  return parse(value);
}

/** The calendar day a value falls on, as a local-midnight Date. */
export function calendarDayOf(value?: string | null): Date | null {
  const instant = parse(value);
  if (!instant) return null;
  return isPickedDay(value)
    ? new Date(instant.getUTCFullYear(), instant.getUTCMonth(), instant.getUTCDate())
    : new Date(instant.getFullYear(), instant.getMonth(), instant.getDate());
}

/** What a date-only picker stores for a chosen day: its midnight UTC. */
export function dayAsUtcMidnight(day: Date): string {
  return new Date(Date.UTC(day.getFullYear(), day.getMonth(), day.getDate())).toISOString();
}
