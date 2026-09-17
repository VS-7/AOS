import { PlayCircle, StopCircle } from "lucide-react";
import type { RoutineStatus } from "@/features/routine/interfaces/routine.interfaces";
import { t } from "@/lib/i18n";

/**
 * The two statuses Go has (`routine.Status`). A third, "paused", was offered
 * on every status menu and filter and refused every time with
 * AOS_ROUTINE_INVALID_STATUS; "disabled" is what it would have meant.
 */
export const ROUTINE_STATUS_ORDER: RoutineStatus[] = ["enabled", "disabled"];

export const ROUTINE_STATUS_CONFIG: Record<
  RoutineStatus,
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
  disabled: {
    get label() { return t("Disabled"); },
    icon: StopCircle,
    color: "text-muted-foreground",
  },
};
