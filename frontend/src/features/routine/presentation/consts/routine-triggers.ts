import type { LucideIcon } from "lucide-react";
import { ActivityIcon, ClockIcon, WebhookIcon } from "lucide-react";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import type { RoutineActivityFilter } from "@/features/routine/interfaces/routine.interfaces";

export type RoutineTriggerTypeId = Routine["triggers"][number]["type"];

export type RoutineScheduledPresetId =
  | "hourly"
  | "daily"
  | "weekly"
  | "custom";

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
      config: {
        token?: string;
      };
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
  addLabel: string;
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
  { id: "hourly", label: "Hourly" },
  { id: "daily", label: "Daily" },
  { id: "weekly", label: "Weekly" },
  { id: "custom", label: "Custom (cron)" },
];

export const ROUTINE_TRIGGER_TYPE_REGISTRY: Record<
  RoutineTriggerTypeId,
  RoutineTriggerTypeDefinition
> = {
  scheduled: {
    id: "scheduled",
    label: "Scheduled",
    addLabel: "Scheduled",
    description: "Run this routine on a recurring schedule.",
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
    label: "Webhook triggered",
    addLabel: "Webhook triggered",
    description: "Fire this routine from an external HTTP request.",
    icon: WebhookIcon,
    searchableTerms: ["webhook", "http", "url", "api"],
    createDefault: () => ({
      type: "webhook",
      config: {
        token: "",
      },
    }),
  },
  activity: {
    id: "activity",
    label: "On activity",
    addLabel: "On activity",
    description: "Fire when a workspace activity event matches.",
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
