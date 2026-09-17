import type { LucideIcon } from "lucide-react";
import { ActivityIcon, ClockIcon, WebhookIcon } from "lucide-react";
import type {
  RoutineActivityFilter,
  RoutineTrigger,
} from "@/features/routine/interfaces/routine.interfaces";
import { t } from "@/lib/i18n";

export type RoutineTriggerTypeId = RoutineTrigger["type"];

export type RoutineScheduledPresetId =
  | "hourly"
  | "daily"
  | "weekly"
  | "custom";

/**
 * A trigger as the editor holds it. A schedule keeps the preset, time and day
 * it was built from beside the cron; the other two carry only what a person
 * chooses. A webhook's secret is the daemon's and never passes through here.
 */
export type RoutineTriggerFormValue =
  | {
      type: "scheduled";
      config: {
        preset: RoutineScheduledPresetId;
        cron: string;
        time: string;
        day: string;
      };
    }
  | {
      type: "webhook";
      config: Record<string, never>;
    }
  | {
      type: "activity";
      config: {
        namespace: string;
        event: string;
        filters?: RoutineActivityFilter[];
      };
    };

export interface RoutineTriggerTypeDefinition {
  id: RoutineTriggerTypeId;
  label: string;
  description: string;
  icon: LucideIcon;
  searchableTerms: string[];
  createDefault: (
    preset?: RoutineScheduledPresetId,
  ) => RoutineTriggerFormValue;
}

export const ROUTINE_SCHEDULED_PRESET_OPTIONS: Array<{
  id: RoutineScheduledPresetId;
  label: string;
}> = [
  { id: "hourly", get label() { return t("Hourly"); } },
  { id: "daily", get label() { return t("Daily"); } },
  { id: "weekly", get label() { return t("Weekly"); } },
  { id: "custom", get label() { return t("Custom (cron)"); } },
];

export const ROUTINE_TRIGGER_TYPE_REGISTRY: Record<
  RoutineTriggerTypeId,
  RoutineTriggerTypeDefinition
> = {
  scheduled: {
    id: "scheduled",
    get label() { return t("Scheduled"); },
    get description() { return t("Run this routine on a recurring schedule."); },
    icon: ClockIcon,
    searchableTerms: ["cron", "schedule", "hourly", "daily", "weekly"],
    createDefault: (preset = "hourly") => ({
      type: "scheduled",
      config: {
        preset,
        cron:
          preset === "hourly"
            ? "0 * * * *"
            : preset === "daily"
              ? "0 9 * * *"
              : preset === "weekly"
                ? "0 9 * * 1"
                : "0 12 1 * *",
        time: "09:00",
        day: "1",
      },
    }),
  },
  webhook: {
    id: "webhook",
    get label() { return t("Webhook triggered"); },
    get description() { return t("Fire this routine from an external HTTP request."); },
    icon: WebhookIcon,
    searchableTerms: ["webhook", "http", "url", "api"],
    createDefault: () => ({ type: "webhook", config: {} }),
  },
  activity: {
    id: "activity",
    get label() { return t("On activity"); },
    get description() { return t("Fire when a workspace activity event matches."); },
    icon: ActivityIcon,
    searchableTerms: ["activity", "event", "task", "chat", "notification"],
    createDefault: () => ({
      type: "activity",
      config: {
        namespace: "",
        event: "",
        filters: [],
      },
    }),
  },
};

export const ROUTINE_TRIGGER_TYPE_ORDER: RoutineTriggerTypeId[] = [
  "scheduled",
  "webhook",
  "activity",
];
