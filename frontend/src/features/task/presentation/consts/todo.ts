import { CircleDashed, Ban, Check, SkipForward } from "lucide-react";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";
import { DotmSquare4 } from "@/components/ui/dotm-square-4";
import { t } from "@/lib/i18n";

// Go's todo lifecycle (`internal/domain/todo/entity.go`) is `pending,
// in_progress, blocked, finished, skipped` — the source had `todo,
// in_progress, in_review, finished` (`in_review` belongs to the *task*
// lifecycle, not a todo's; Go's todo has no such state). See
// `interfaces/task.interfaces.ts`'s `TodoStatusSchema` doc comment.
export const TODO_STATUS_CONFIG: Record<Todo["status"], { label: string; icon: any; color: string }> = {
  pending: { get label() { return t("Pending"); }, icon: CircleDashed, color: "text-muted-foreground" },
  in_progress: { get label() { return t("In Progress"); }, icon: DotmSquare4, color: "text-primary" },
  blocked: { get label() { return t("Blocked"); }, icon: Ban, color: "text-destructive" },
  finished: { get label() { return t("Finished"); }, icon: Check, color: "text-success" },
  skipped: { get label() { return t("Skipped"); }, icon: SkipForward, color: "text-muted-foreground" },
};

/**
 * Where a step can move from each status: internal/domain/todo/entity.go's
 * `transitions`, which refuses anything else. A test reads that file and
 * fails when the two disagree.
 */
export const TODO_TRANSITIONS: Record<Todo["status"], Todo["status"][]> = {
  pending: ["in_progress", "blocked", "finished", "skipped"],
  in_progress: ["blocked", "finished", "pending", "skipped"],
  blocked: ["in_progress", "pending", "skipped"],
  finished: ["pending", "in_progress"],
  skipped: ["pending"],
};

export const TODO_STATUS_ORDER: Todo["status"][] = ["pending", "in_progress", "blocked", "finished", "skipped"];
