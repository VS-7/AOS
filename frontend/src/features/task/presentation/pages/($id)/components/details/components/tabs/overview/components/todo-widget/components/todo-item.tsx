import React, { useRef, useState } from "react";
import { Bot } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { t } from "@/lib/i18n";

interface TodoItemProps {
  todo: Todo;
}

export function TodoItem({ todo }: TodoItemProps) {
  const [open, setOpen] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const statusCfg = TaskHelper.getTodoStatus(todo.status);
  const StatusIcon = statusCfg.icon;
  const statusColor = statusCfg.color;

  const isFinished = todo.status === "finished";

  function handleMouseEnterTrigger() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
    setOpen(true);
  }

  function handleMouseLeaveTrigger() {
    closeTimer.current = setTimeout(() => setOpen(false), 200);
  }

  function handleMouseEnterContent() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
  }

  function handleMouseLeaveContent() {
    closeTimer.current = setTimeout(() => setOpen(false), 200);
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <div
          className="cursor-pointer"
          onMouseEnter={handleMouseEnterTrigger}
          onMouseLeave={handleMouseLeaveTrigger}
        >
          <SplitPageLayout.WidgetItem className="group relative pr-7">
            <StatusIcon className={`size-3.5 shrink-0 mt-0.5 ${statusColor}`} />
            {/* Go's `Todo` field is `title`, not `description` — see
                `interfaces/task.interfaces.ts`'s `TaskTodoSchema`. */}
            <span className={`text-xs leading-snug line-clamp-1 flex-1 ${isFinished ? "line-through text-muted-foreground" : ""}`}>
              {todo.title}
            </span>
          </SplitPageLayout.WidgetItem>
        </div>
      </PopoverTrigger>
      <PopoverContent
        side="left"
        align="start"
        className="w-72 p-3 space-y-2.5"
        onMouseEnter={handleMouseEnterContent}
        onMouseLeave={handleMouseLeaveContent}
      >
        <p className="text-xs font-medium leading-snug">{todo.title}</p>
        <div className="flex items-center gap-2">
          <StatusIcon className={`size-3 shrink-0 ${statusColor}`} />
          <span className="text-sm capitalize text-muted-foreground">
            {todo.status.replace(/_/g, " ")}
          </span>
        </div>
        {todo.agent && (
          <div className="flex items-center gap-2">
            <Bot className="size-3 shrink-0 text-muted-foreground" />
            <Badge variant="secondary" className="text-sm h-4 px-1.5">{todo.agent}</Badge>
          </div>
        )}
        {/* Go's field is `content`, not `instructions` — same schema note. */}
        {todo.content && (
          <div className="space-y-1">
            <span className="text-sm text-muted-foreground">{t("Notes")}</span>
            <p className=" truncate">{todo.content}</p>
          </div>
        )}
        {todo.output && Object.keys(todo.output).length > 0 && (
          <div className="space-y-1">
            <span className="text-sm text-muted-foreground">{t("Output")}</span>
            <pre className="text-sm font-mono bg-muted/40 rounded p-2 overflow-auto max-h-24">
              {JSON.stringify(todo.output, null, 2)}
            </pre>
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
