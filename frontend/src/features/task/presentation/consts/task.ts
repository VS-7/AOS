import { CircleDashed, CheckCircle2, SignalZero, SignalLow, SignalMedium, SignalHigh, FlagIcon, CirclePauseIcon, CirclePlayIcon, CircleDotDashedIcon, CircleDashedIcon, CircleFadingPlusIcon } from "lucide-react";
import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import { t } from "@/lib/i18n";

export const TASK_STATUS_CONFIG: Record<Task["status"], { label: string; icon: any; color: string; defaultOpen?: boolean }> = {
  suggestion: { get label() { return t("Suggestion"); }, icon: CircleFadingPlusIcon, color: "text-muted-foreground" },
  backlog: { get label() { return t("Backlog"); }, icon: CircleDashedIcon, color: "text-muted-foreground" },
  planning: { get label() { return t("Planning"); }, icon: CircleDotDashedIcon, color: "text-primary" },
  todo: { get label() { return t("Todo"); }, icon: CircleDashed, color: "text-muted-foreground" },
  in_progress: { get label() { return t("In Progress"); }, icon: CirclePlayIcon, color: "text-primary" },
  stopped: { get label() { return t("Stopped"); }, icon: CircleFadingPlusIcon, color: "text-warning" },
  in_review: { get label() { return t("In Review"); }, icon: CirclePauseIcon, color: "text-warning" },
  finished: { get label() { return t("Finished"); }, icon: CheckCircle2, color: "text-success", defaultOpen: false },
};

export const TASK_PRIORITY_CONFIG: Record<TaskPriority, { label: string; icon: any; colorClass: string }> = {
  no_priority: { get label() { return t("No Priority"); }, icon: SignalZero, colorClass: "text-muted-foreground/70" },
  low: { get label() { return t("Low"); }, icon: SignalLow, colorClass: "text-muted-foreground/70" },
  medium: { get label() { return t("Medium"); }, icon: SignalMedium, colorClass: "text-muted-foreground/70" },
  high: { get label() { return t("High"); }, icon: SignalHigh, colorClass: "text-yellow-500" },
  urgent: { get label() { return t("Urgent"); }, icon: FlagIcon, colorClass: "text-red-500" },
};

export const TASK_STATUS_ORDER: Task["status"][] = ["suggestion", "backlog", "planning", "todo", "in_progress", "stopped", "in_review", "finished"];
export const TASK_PRIORITY_ORDER: TaskPriority[] = ["no_priority", "urgent", "high", "medium", "low"];
