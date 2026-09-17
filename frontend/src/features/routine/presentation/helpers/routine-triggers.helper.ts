import { z } from "zod";
import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { ActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import type {
  Routine,
  RoutineTriggerInput,
} from "@/features/routine/interfaces/routine.interfaces";
import {
  type RoutineScheduledPresetId,
  type RoutineTriggerFormValue,
  type RoutineTriggerTypeId,
} from "@/features/routine/presentation/consts/routine-triggers";
import { getLocale, t } from "@/lib/i18n";

export type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";

export const RoutineScheduledPresetSchema = z.enum([
  "hourly",
  "daily",
  "weekly",
  "custom",
]);

// Messages are catalogue keys: FormMessage translates what it renders.
export const RoutineActivityFilterFormSchema = z.object({
  field: z.string().refine((value) => value.trim() !== "", "Field is required"),
  operator: z.enum(["eq", "neq", "contains"]),
  value: z.string(),
});

/**
 * The shape the daemon parses, checked here only as far as counting fields:
 * a cron that is not five of them cannot be right, and saying so beside the
 * input beats a refusal after Save. The daemon still has the last word on
 * what each field may hold.
 */
const cronSchema = z
  .string()
  .refine((value) => value.trim() !== "", "Cron expression is required")
  .refine(
    (value) => value.trim() === "" || value.trim().split(/\s+/).length === 5,
    "A cron expression has five fields: minute, hour, day of month, month and day of week",
  );

export const RoutineTriggerFormSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("scheduled"),
    config: z.object({
      preset: RoutineScheduledPresetSchema,
      cron: cronSchema,
      time: z.string().default("09:00"),
      day: z.string().default("1"),
    }),
  }),
  z.object({
    type: z.literal("webhook"),
    config: z.object({}).strip(),
  }),
  z.object({
    type: z.literal("activity"),
    config: z.object({
      namespace: z.string().min(1, "Namespace is required"),
      event: z.string().min(1, "Event is required"),
      filters: z.array(RoutineActivityFilterFormSchema).optional(),
    }),
  }),
]);

export type RoutineTriggerFormInput = z.infer<typeof RoutineTriggerFormSchema>;

export const ROUTINE_WEEKDAY_OPTIONS = [
  { value: "1", get label() { return t("Monday"); } },
  { value: "2", get label() { return t("Tuesday"); } },
  { value: "3", get label() { return t("Wednesday"); } },
  { value: "4", get label() { return t("Thursday"); } },
  { value: "5", get label() { return t("Friday"); } },
  { value: "6", get label() { return t("Saturday"); } },
  { value: "0", get label() { return t("Sunday"); } },
] as const;

/** Every half hour of the day, the choices a new schedule is offered. */
const HALF_HOURS = Array.from({ length: 48 }, (_, index) => {
  const hour = Math.floor(index / 2)
    .toString()
    .padStart(2, "0");
  const minute = index % 2 === 0 ? "00" : "30";
  return `${hour}:${minute}`;
});

export class RoutineTriggersHelper {
  /** A stored routine's triggers, as the editor holds them. */
  public static buildFormTriggers(
    routine: Routine | null,
  ): RoutineTriggerFormValue[] {
    if (!routine) return [];

    return routine.triggers.map((trigger): RoutineTriggerFormValue => {
      if (trigger.type === "webhook") {
        return { type: "webhook", config: {} };
      }

      if (trigger.type === "activity") {
        return {
          type: "activity",
          config: {
            namespace: trigger.config.namespace,
            event: trigger.config.event ?? "",
            // Beside `config`, not inside it: that is where Go keeps them.
            filters: (trigger.filters ?? []).map((filter) => ({
              field: filter.field,
              operator: filter.operator,
              value:
                typeof filter.value === "string"
                  ? filter.value
                  : JSON.stringify(filter.value ?? ""),
            })),
          },
        };
      }

      const inferred = this.inferScheduledConfig(trigger.config.cron);
      return {
        type: "scheduled",
        config: {
          cron: trigger.config.cron,
          ...inferred,
        },
      };
    });
  }

  /**
   * The editor's triggers as `routine.TriggerInput`, the flat shape create
   * and update take. A webhook carries nothing: the daemon keeps the token a
   * routine already has, and mints one only for a webhook that is new.
   */
  public static toApiTriggers(
    triggers: RoutineTriggerFormValue[],
  ): RoutineTriggerInput[] {
    return triggers.map((trigger): RoutineTriggerInput => {
      if (trigger.type === "webhook") {
        return { type: "webhook" };
      }

      if (trigger.type === "activity") {
        const filters = trigger.config.filters ?? [];
        return {
          type: "activity",
          namespace: trigger.config.namespace,
          event: trigger.config.event,
          ...(filters.length > 0
            ? {
                filters: filters.map((filter) => ({
                  field: filter.field.trim(),
                  operator: filter.operator,
                  value: filter.value,
                })),
              }
            : {}),
        };
      }

      return { type: "scheduled", cron: trigger.config.cron };
    });
  }

  public static inferScheduledConfig(cron: string): {
    preset: RoutineScheduledPresetId;
    time: string;
    day: string;
  } {
    if (cron.trim() === "0 * * * *") {
      return { preset: "hourly", time: "09:00", day: "1" };
    }

    const dailyMatch = cron.trim().match(/^(\d{1,2})\s+(\d{1,2})\s+\*\s+\*\s+\*$/);
    if (dailyMatch && this._isClockTime(dailyMatch[2], dailyMatch[1])) {
      return {
        preset: "daily",
        time: this._formatTime(dailyMatch[2], dailyMatch[1]),
        day: "1",
      };
    }

    const weeklyMatch = cron.trim().match(/^(\d{1,2})\s+(\d{1,2})\s+\*\s+\*\s+([0-6])$/);
    if (weeklyMatch && this._isClockTime(weeklyMatch[2], weeklyMatch[1])) {
      return {
        preset: "weekly",
        time: this._formatTime(weeklyMatch[2], weeklyMatch[1]),
        day: weeklyMatch[3],
      };
    }

    return { preset: "custom", time: "09:00", day: "1" };
  }

  /**
   * The times a schedule's picker offers: every half hour, plus the one it
   * already has. A cron written elsewhere at :15 is still a daily or weekly
   * schedule, and a picker without its time showed it blank.
   */
  public static timeOptions(current: string): string[] {
    if (!current || HALF_HOURS.includes(current)) return HALF_HOURS;
    return [...HALF_HOURS, current].sort();
  }

  public static buildCronFromScheduledConfig(config: {
    preset: RoutineScheduledPresetId;
    time: string;
    day: string;
    cron?: string;
  }): string {
    if (config.preset === "custom") {
      return config.cron?.trim() || "0 9 * * *";
    }

    if (config.preset === "hourly") {
      return "0 * * * *";
    }

    const [hour, minute] = config.time.split(":").map((part) => Number(part));

    if (config.preset === "daily") {
      return `${minute} ${hour} * * *`;
    }

    return `${minute} ${hour} * * ${config.day}`;
  }

  public static getScheduledSummary(config: {
    preset: RoutineScheduledPresetId;
    time: string;
    day: string;
    cron: string;
  }): string {
    if (config.preset === "hourly") return t("Every hour");
    if (config.preset === "daily") return t("Every day");
    if (config.preset === "weekly") {
      const dayLabel =
        ROUTINE_WEEKDAY_OPTIONS.find((option) => option.value === config.day)
          ?.label ?? t("Monday");
      return t("Every week on {{day}}", { day: dayLabel });
    }

    return t("Custom schedule");
  }

  /** A moment, in the interface's language rather than the machine's. */
  public static formatNextRun(at: Date): string {
    const formatter = new Intl.DateTimeFormat(getLocale(), {
      weekday: "short",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      timeZoneName: "short",
    });
    return t("Next run {{when}}", { when: formatter.format(at) });
  }

  /**
   * When a schedule fires next.
   *
   * The daemon's answer (`routine.nextRun`) is the one to show for a schedule
   * that is saved: it evaluates any cron, not just the presets. `savedNextRun`
   * is passed only when the cron on screen is the saved one; an edit not yet
   * saved gets the local calculation, which covers the presets.
   */
  public static getNextRunLabel(cron: string, savedNextRun?: string): string | null {
    if (savedNextRun) {
      const at = new Date(savedNextRun);
      if (!Number.isNaN(at.getTime())) return this.formatNextRun(at);
    }

    const inferred = this.inferScheduledConfig(cron);
    const nextRunAt = this._resolve_next_occurrence(inferred, new Date());
    return nextRunAt ? this.formatNextRun(nextRunAt) : null;
  }

  /**
   * The daemon's warnings about a routine, in the interface's language when
   * they are ones it knows. They arrive as English sentences; the two the
   * scheduler writes are recognised and said again here, and anything else
   * is shown as the daemon wrote it rather than hidden.
   */
  public static describeWarnings(routine: Pick<Routine, "warnings" | "effectiveInterval"> | null): string[] {
    return (routine?.warnings ?? []).map((warning) => {
      if (warning.includes("more often than")) {
        return t(
          "This schedule is finer than the scheduler's {{interval}} tick, so it fires once per tick. That is the real resolution of the system.",
          { interval: this.humanInterval(routine?.effectiveInterval) },
        );
      }
      if (warning.includes("does not parse")) {
        return t("This cron expression does not parse, so the routine never fires on a schedule.");
      }
      return warning;
    });
  }

  /** Go's duration string ("15m0s", "1h0m0s") as a person writes it. */
  public static humanInterval(interval?: string): string {
    if (!interval) return "15 min";
    const hours = Number(interval.match(/(\d+)h/)?.[1] ?? 0);
    const minutes = Number(interval.match(/(\d+)m/)?.[1] ?? 0);
    if (hours > 0 && minutes > 0) return `${hours} h ${minutes} min`;
    if (hours > 0) return `${hours} h`;
    return `${minutes} min`;
  }

  /**
   * Computes the next scheduled occurrence for a preset-based cron expression.
   *
   * @param config - Inferred preset configuration (preset, time, day).
   * @param now - Reference time used as the lower bound for the next run.
   * @returns Next occurrence `Date`, or `null` for `custom` presets.
   */
  private static _resolve_next_occurrence(
    config: { preset: RoutineScheduledPresetId; time: string; day: string },
    now: Date,
  ): Date | null {
    if (config.preset === "custom") {
      return null;
    }

    if (config.preset === "hourly") {
      const next = new Date(now);
      next.setMinutes(0, 0, 0);
      next.setHours(next.getHours() + 1);
      return next;
    }

    const [hour, minute] = config.time.split(":").map((part) => Number(part));
    if (Number.isNaN(hour) || Number.isNaN(minute)) {
      return null;
    }

    if (config.preset === "daily") {
      return this._next_weekly_occurrence(now, hour, minute, [0, 1, 2, 3, 4, 5, 6]);
    }

    if (config.preset === "weekly") {
      const dayNumber = Number(config.day);
      if (Number.isNaN(dayNumber)) {
        return null;
      }
      return this._next_weekly_occurrence(now, hour, minute, [dayNumber]);
    }

    return null;
  }

  /**
   * Resolves the next occurrence of `(hour, minute)` allowed by the given weekdays.
   *
   * @param now - Reference time used as the lower bound.
   * @param hour - Local hour of the day (0-23).
   * @param minute - Local minute of the hour (0-59).
   * @param allowedDays - Weekday numbers (0=Sunday, 1=Monday, ..., 6=Saturday).
   * @returns Next valid `Date` strictly after `now`.
   */
  private static _next_weekly_occurrence(
    now: Date,
    hour: number,
    minute: number,
    allowedDays: number[],
  ): Date | null {
    for (let offset = 0; offset <= 7; offset += 1) {
      const candidate = new Date(now);
      candidate.setDate(candidate.getDate() + offset);
      candidate.setHours(hour, minute, 0, 0);

      if (!allowedDays.includes(candidate.getDay())) {
        continue;
      }

      if (candidate.getTime() <= now.getTime()) {
        continue;
      }

      return candidate;
    }

    return null;
  }

  public static countScheduledTriggers(
    triggers: RoutineTriggerFormValue[],
  ): number {
    return triggers.filter((trigger) => trigger.type === "scheduled").length;
  }

  public static getUsedActivityEventKeys(
    triggers: RoutineTriggerFormValue[],
  ): Set<string> {
    const keys = new Set<string>();

    for (const trigger of triggers) {
      if (trigger.type === "activity" && trigger.config.namespace && trigger.config.event) {
        keys.add(
          ActivityEventHelper.getEventKey(
            trigger.config.namespace,
            trigger.config.event,
          ),
        );
      }
    }

    return keys;
  }

  public static getAvailableActivityEvents(
    triggers: RoutineTriggerFormValue[],
    eventDefinitions: ActivityEventDefinition[],
  ): ActivityEventDefinition[] {
    return ActivityEventHelper.getAvailableDefinitions(
      eventDefinitions,
      this.getUsedActivityEventKeys(triggers),
    );
  }

  public static getAvailableTriggerTypes(
    triggers: RoutineTriggerFormValue[],
    eventDefinitions: ActivityEventDefinition[],
  ): RoutineTriggerTypeId[] {
    const activeTypes = new Set(
      triggers
        .filter((trigger) => trigger.type !== "activity")
        .map((trigger) => trigger.type),
    );

    const available: RoutineTriggerTypeId[] = [];

    if (!activeTypes.has("scheduled")) {
      available.push("scheduled");
    }

    if (!activeTypes.has("webhook")) {
      available.push("webhook");
    }

    if (this.getAvailableActivityEvents(triggers, eventDefinitions).length > 0) {
      available.push("activity");
    }

    return available;
  }

  public static canAddTriggers(
    triggers: RoutineTriggerFormValue[],
    eventDefinitions: ActivityEventDefinition[],
  ): boolean {
    return this.getAvailableTriggerTypes(triggers, eventDefinitions).length > 0;
  }

  public static isActivityTriggerDuplicate(
    a: { namespace: string; event: string },
    b: { namespace: string; event: string },
  ): boolean {
    return a.namespace === b.namespace && a.event === b.event;
  }

  /**
   * An activity event's name for a person: its title, in the interface's
   * language when the catalogue has it, and the raw key only when the daemon
   * gave no title at all.
   */
  public static eventTitle(definition: ActivityEventDefinition | undefined, namespace: string, event: string): string {
    const title = definition?.title?.trim();
    if (title) return t(title);
    return event ? `${namespace} · ${event.replace(/_/g, " ")}` : namespace;
  }

  /** The longer sentence under an event's title, translated when catalogued. */
  public static eventDescription(definition: ActivityEventDefinition | undefined): string {
    const description = definition?.description?.trim();
    return description ? t(description) : "";
  }

  private static _isClockTime(hour: string, minute: string): boolean {
    return Number(hour) <= 23 && Number(minute) <= 59;
  }

  private static _formatTime(hour: string, minute: string): string {
    return `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
  }
}
