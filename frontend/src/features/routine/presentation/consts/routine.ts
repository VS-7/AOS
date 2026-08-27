import { PauseCircle, PlayCircle, StopCircle } from "lucide-react";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import type { RoutineReservedAgent } from "@/features/routine/interfaces/routine.interfaces";
import { t } from "@/lib/i18n";

export const ROUTINE_RESERVED_AGENT_CONFIG: Record<
  RoutineReservedAgent,
  { label: string; description: string }
> = {
  orchestrator: {
    get label() { return t("Orchestrator"); },
    get description() { return t("Resolves to the workspace orchestrator agent at fire time."); },
  },
  all: {
    get label() { return t("All agents"); },
    get description() { return t("Manual fire only — runs once per workspace agent."); },
  },
};

export const ROUTINE_STATUS_ORDER: Routine["status"][] = [
  "enabled",
  "paused",
  "disabled",
];

export const ROUTINE_STATUS_CONFIG: Record<
  Routine["status"],
  {
    label: string;
    icon: typeof PlayCircle;
    color: string;
  }
> = {
  enabled: {
    get label() { return t("Enabled"); },
    icon: PlayCircle,
    color: "text-emerald-600",
  },
  // Task 9 addition: `RoutineStatusSchema` (routine.interfaces.ts)
  // already had "paused" — this consts file's `enabled`/`disabled` pair
  // predates that and was missing the third value.
  paused: {
    get label() { return t("Paused"); },
    icon: PauseCircle,
    color: "text-amber-600",
  },
  disabled: {
    get label() { return t("Disabled"); },
    icon: StopCircle,
    color: "text-muted-foreground",
  },
};
