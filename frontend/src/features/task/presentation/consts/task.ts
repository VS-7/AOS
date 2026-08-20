import { CircleDashed, CheckCircle2, SignalZero, SignalLow, SignalMedium, SignalHigh, FlagIcon, CirclePauseIcon, CirclePlayIcon, CircleDotDashedIcon, CircleDashedIcon, CircleFadingPlusIcon } from "lucide-react";
import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";

export const TASK_STATUS_CONFIG: Record<Task["status"], { label: string; icon: any; color: string; defaultOpen?: boolean }> = {
  suggestion: { label: "Suggestion", icon: CircleFadingPlusIcon, color: "text-muted-foreground" },
  backlog: { label: "Backlog", icon: CircleDashedIcon, color: "text-muted-foreground" },
  planning: { label: "Planning", icon: CircleDotDashedIcon, color: "text-primary" },
  todo: { label: "Todo", icon: CircleDashed, color: "text-muted-foreground" },
  in_progress: { label: "In Progress", icon: CirclePlayIcon, color: "text-primary" },
  stopped: { label: "Stopped", icon: CircleFadingPlusIcon, color: "text-warning" },
  in_review: { label: "In Review", icon: CirclePauseIcon, color: "text-warning" },
  finished: { label: "Finished", icon: CheckCircle2, color: "text-success", defaultOpen: false },
};

export const TASK_PRIORITY_CONFIG: Record<TaskPriority, { label: string; icon: any; colorClass: string }> = {
  no_priority: { label: "No Priority", icon: SignalZero, colorClass: "text-muted-foreground/70" },
  low: { label: "Low", icon: SignalLow, colorClass: "text-muted-foreground/70" },
  medium: { label: "Medium", icon: SignalMedium, colorClass: "text-muted-foreground/70" },
  high: { label: "High", icon: SignalHigh, colorClass: "text-yellow-500" },
  urgent: { label: "Urgent", icon: FlagIcon, colorClass: "text-red-500" },
};

export const TASK_STATUS_ORDER: Task["status"][] = ["suggestion", "backlog", "planning", "todo", "in_progress", "stopped", "in_review", "finished"];
export const TASK_PRIORITY_ORDER: TaskPriority[] = ["no_priority", "urgent", "high", "medium", "low"];
