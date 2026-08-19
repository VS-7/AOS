import { CircleDashed, Eye, Check } from "lucide-react";
import type { FractalTodo } from "@/features/task/interfaces/todo.interfaces";
import { DotmSquare4 } from "@/components/ui/dotm-square-4";

export const TODO_STATUS_CONFIG: Record<FractalTodo["status"], { label: string; icon: any; color: string }> = {
  todo: { label: "Todo", icon: CircleDashed, color: "text-muted-foreground" },
  in_progress: { label: "In Progress", icon: DotmSquare4, color: "text-primary" },
  in_review: { label: "In Review", icon: Eye, color: "text-warning" },
  finished: { label: "Finished", icon: Check, color: "text-success" },
};
