import React from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { FractalTask } from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { TaskKanbanCard } from "./task-kanban-card.component";

import { useDroppable } from "@dnd-kit/core";

interface TaskKanbanColumnProps {
  status: FractalTask["status"];
  tasks: FractalTask[];
  draggedTaskId: FractalTask["id"] | null;
  isDragActive: boolean;
  isActiveDropTarget: boolean;
}

export const TaskKanbanColumn = React.memo(function TaskKanbanColumn({
  status,
  tasks,
  draggedTaskId,
  isDragActive,
  isActiveDropTarget,
}: TaskKanbanColumnProps) {
  const config = TaskHelper.getStatus(status);
  const Icon = config.icon;
  const isEmpty = tasks.length === 0;

  const { setNodeRef } = useDroppable({
    id: status,
  });

  return (
    <section
      className={cn(
        "flex h-full min-h-0 w-80 flex-col p-3 transition-colors",
        isActiveDropTarget && "bg-accent/20",
      )}
    >
      <header
        className={cn(
          "mb-3 flex items-center gap-2 rounded-lg px-2 py-1 transition-colors",
          isActiveDropTarget && "bg-background/80",
        )}
      >
        <Icon className={`size-4 ${config.color}`} />
        <h2 className="text-sm font-medium">{config.label}</h2>
        <Badge variant="secondary" className="h-5 px-1.5 text-xs">
          {tasks.length}
        </Badge>
      </header>

      <div
        ref={setNodeRef}
        className={cn(
          "flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto rounded-xl border border-transparent p-1 pr-2 transition-[border-color,background-color,box-shadow]",
          isDragActive && "border-dashed border-border/80 bg-muted/25",
          isActiveDropTarget &&
            "border-primary/35 bg-accent/30 shadow-inner ring-1 ring-primary/10",
          isEmpty && "justify-center",
        )}
      >
        {tasks.map((task) => (
          <TaskKanbanCard
            key={task.id}
            task={task}
            isDragging={draggedTaskId === task.id}
          />
        ))}

        {isEmpty && (
          <div
            className={cn(
              "flex h-full items-center justify-center rounded-lg border border-dashed px-3 py-6 text-center text-xs text-muted-foreground transition-colors",
            )}
          >
            {isActiveDropTarget
              ? "Drop task here"
              : `No tasks in ${config.label.toLowerCase()}`}
          </div>
        )}
      </div>
    </section>
  );
});
