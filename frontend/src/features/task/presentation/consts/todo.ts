import { CircleDashed, Ban, Check, SkipForward } from "lucide-react";
import type { FractalTodo } from "@/features/task/interfaces/todo.interfaces";
import { DotmSquare4 } from "@/components/ui/dotm-square-4";

// Go's todo lifecycle (`internal/domain/todo/entity.go`) is `pending,
// in_progress, blocked, finished, skipped` — the source had `todo,
// in_progress, in_review, finished` (`in_review` belongs to the *task*
// lifecycle, not a todo's; Go's todo has no such state). See
// `interfaces/task.interfaces.ts`'s `FractalTodoStatusSchema` doc comment.
export const TODO_STATUS_CONFIG: Record<FractalTodo["status"], { label: string; icon: any; color: string }> = {
  pending: { label: "Pending", icon: CircleDashed, color: "text-muted-foreground" },
  in_progress: { label: "In Progress", icon: DotmSquare4, color: "text-primary" },
  blocked: { label: "Blocked", icon: Ban, color: "text-destructive" },
  finished: { label: "Finished", icon: Check, color: "text-success" },
  skipped: { label: "Skipped", icon: SkipForward, color: "text-muted-foreground" },
};
