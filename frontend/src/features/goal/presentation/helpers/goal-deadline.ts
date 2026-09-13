import { getLocale, type Locale } from "@/lib/i18n";
import { calendarDayOf, dayAsUtcMidnight, instantOf, isPickedDay } from "@/lib/calendar-day";

/**
 * A goal's deadline is a day, and the daemon keeps it as an instant.
 *
 * The goal page's picker stores the chosen day as midnight UTC, and an agent
 * may set a real time of day; `lib/calendar-day.ts` reads both back as the
 * right day. Every place that shows a goal deadline — the list, the project's
 * Goals tab, the goal page — goes through here, which is what keeps them
 * agreeing: the list used to write the day in local time with a hard-coded
 * en-US locale ("Sep 19, 2026" for a goal due on the 20th, in English inside
 * the Portuguese interface) while the goal page read the UTC day.
 */

/** The calendar day a deadline falls on, as a local-midnight Date. */
export const deadlineDay = calendarDayOf;

/** What the picker stores for a chosen day. */
export const dayToDeadline = dayAsUtcMidnight;

/** The deadline's day, written in the interface's language. */
export function formatDeadline(value?: string | null, locale: Locale = getLocale()): string | null {
  const day = calendarDayOf(value);
  if (!day) return null;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(day);
}

/**
 * Whether the deadline has passed: a picked day once it is over, an instant
 * once it is behind us.
 */
export function isDeadlineOverdue(value?: string | null, now: Date = new Date()): boolean {
  const instant = instantOf(value);
  if (!instant) return false;
  if (!isPickedDay(instant)) return instant.getTime() < now.getTime();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return calendarDayOf(value)!.getTime() < today.getTime();
}
