import React from "react";
import { useDraggable } from "@dnd-kit/core";
import { GripVertical } from "lucide-react";
import type { FractalTask } from "@/features/task/interfaces/task.interfaces";
import { TaskListRow } from "./task-list-row.component";

interface DraggableTaskListRowProps {
  task: FractalTask;
}

export const DraggableTaskListRow = React.memo(function DraggableTaskListRow({
  task,
}: DraggableTaskListRowProps) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: task.id,
  });

  const dragHandle = (
    <div
      {...attributes}
      {...listeners}
      className="flex cursor-grab items-center justify-center p-1 text-muted-foreground/30 hover:text-muted-foreground/80 active:cursor-grabbing"
    >
      <GripVertical className="size-3.5" />
    </div>
  );

  return (
    <div ref={setNodeRef}>
      <TaskListRow
        task={task}
        isDragging={isDragging}
        dragHandle={dragHandle}
      />
    </div>
  );
});
