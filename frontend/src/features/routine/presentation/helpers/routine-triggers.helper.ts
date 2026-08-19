import { z } from "zod";
import type { FractalActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import { FractalActivityEventHelper } from "@/features/activity/presentation/helpers/activity-event.helper";
import type { FractalRoutine } from "@/features/routine/interfaces/routine.interfaces";
import {
  ROUTINE_SCHEDULED_PRESET_OPTIONS,
  type RoutineScheduledPresetId,
  type RoutineTriggerFormValue,
  type RoutineTriggerTypeId,
} from "@/features/routine/presentation/consts/routine-triggers";

export type { RoutineTriggerFormValue } from "@/features/routine/presentation/consts/routine-triggers";

export const RoutineScheduledPresetSchema = z.enum([
  "hourly",
  "daily",
  "weekly",
  "custom",
]);

export const RoutineActivityFilterFormSchema = z.object({
  path: z.string().min(1, "Field is required"),
  operator: z.enum(["eq", "neq", "contains"]),
  value: z.string(),
});

export const RoutineTriggerFormSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("scheduled"),
    config: z.object({
      preset: RoutineScheduledPresetSchema,
      cron: z.string().min(1, "Cron expression is required"),
      time: z.string().default("09:00"),
      day: z.string().default("1"),
    }),
  }),
  z.object({
    type: z.literal("webhook"),
    config: z.object({
      token: z.string().optional(),
    }),
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
  { value: "1", label: "Monday" },
  { value: "2", label: "Tuesday" },
  { value: "3", label: "Wednesday" },
  { value: "4", label: "Thursday" },
  { value: "5", label: "Friday" },
  { value: "6", label: "Saturday" },
  { value: "0", label: "Sunday" },
] as const;

export class RoutineTriggersHelper {
  public static buildFormTriggers(
    routine: FractalRoutine | null,
  ): RoutineTriggerFormValue[] {
    if (!routine) return [];

    return routine.triggers.map((trigger) => {
      if (trigger.type === "webhook") {
        return {
          type: "webhook",
          config: {
            token: trigger.config.token,
          },
        };
      }

      if (trigger.type === "activity") {
        return {
          type: "activity",
          config: {
            namespace: trigger.config.namespace,
            event: trigger.config.event,
            filters: trigger.config.filters ?? [],
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

  public static toApiTriggers(
    triggers: RoutineTriggerFormValue[],
    existingRoutine: FractalRoutine | null,
  ): FractalRoutine["triggers"] {
    return triggers.map((trigger) => {
      if (trigger.type === "webhook") {
        const existingToken = existingRoutine?.triggers.find(
          (item) => item.type === "webhook",
        )?.config.token;

        return {
          type: "webhook",
          config: {
            token: trigger.config.token || existingToken || "",
          },
        };
      }

      if (trigger.type === "activity") {
        return {
          type: "activity",
          config: {
            namespace: trigger.config.namespace,
            event: trigger.config.event,
            filters:
              trigger.config.filters && trigger.config.filters.length > 0
                ? trigger.config.filters
                : undefined,
          },
        };
      }

      return {
        type: "scheduled",
        config: {
          cron: trigger.config.cron,
        },
      };
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

    const dailyMatch = cron.match(/^(\d+)\s+(\d+)\s+\*\s+\*\s+\*$/);
    if (dailyMatch) {
      return {
        preset: "daily",
        time: this._formatTime(dailyMatch[2], dailyMatch[1]),
        day: "1",
      };
    }

    const weeklyMatch = cron.match(/^(\d+)\s+(\d+)\s+\*\s+\*\s+(\d)$/);
    if (weeklyMatch) {
      return {
        preset: "weekly",
        time: this._formatTime(weeklyMatch[2], weeklyMatch[1]),
        day: weeklyMatch[3],
      };
    }

    return { preset: "custom", time: "09:00", day: "1" };
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
    if (config.preset === "hourly") return "Every hour";
    if (config.preset === "daily") return "Every day";
    if (config.preset === "weekly") {
      const dayLabel =
        ROUTINE_WEEKDAY_OPTIONS.find((option) => option.value === config.day)
          ?.label ?? "Monday";
      return `Every week on ${dayLabel}`;
    }

    return "Custom schedule";
  }

  public static getNextRunLabel(cron: string): string | null {
    // [Rationale]: Cron evaluation lives in the backend routine automation
    // service. The presentation layer only needs a human-readable label for
    // the three supported presets (`hourly`, `daily`, `weekly`). We avoid
    // pulling `cron-parser` into the frontend bundle and only show a label
    // when the schedule matches one of those presets.
    const inferred = this.inferScheduledConfig(cron);
    const now = new Date();

    const nextRunAt = this._resolve_next_occurrence(inferred, now);
    if (!nextRunAt) {
      return null;
    }

    const formatter = new Intl.DateTimeFormat(undefined, {
      weekday: "short",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      timeZoneName: "short",
    });

    return `Next run ${formatter.format(nextRunAt)}`;
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
          FractalActivityEventHelper.getEventKey(
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
    eventDefinitions: FractalActivityEventDefinition[],
  ): FractalActivityEventDefinition[] {
    return FractalActivityEventHelper.getAvailableDefinitions(
      eventDefinitions,
      this.getUsedActivityEventKeys(triggers),
    );
  }

  public static getAvailableTriggerTypes(
    triggers: RoutineTriggerFormValue[],
    eventDefinitions: FractalActivityEventDefinition[],
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
    eventDefinitions: FractalActivityEventDefinition[],
  ): boolean {
    return this.getAvailableTriggerTypes(triggers, eventDefinitions).length > 0;
  }

  public static isActivityTriggerDuplicate(
    a: { namespace: string; event: string },
    b: { namespace: string; event: string },
  ): boolean {
    return a.namespace === b.namespace && a.event === b.event;
  }

  private static _formatTime(hour: string, minute: string): string {
    return `${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
  }
}
