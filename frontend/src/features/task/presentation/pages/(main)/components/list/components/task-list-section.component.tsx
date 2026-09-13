import React from "react";
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
  CollapsibleTitle,
  CollapsibleIcon,
} from "@/components/ui/collapsible";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { DraggableTaskListRow } from "./draggable-task-list-row.component";
import { useDroppable } from "@dnd-kit/core";
import { useDragContext } from "../../../context";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface TasksListSectionProps {
  status: Task["status"];
  tasks: Task[];
}

export const TasksListSection = React.memo(function TasksListSection({
  status,
  tasks,
}: TasksListSectionProps) {
  const config = TaskHelper.getStatus(status);
  const Icon = config.icon;
  const isEmpty = tasks.length === 0;
  const [isOpen, setIsOpen] = React.useState(!isEmpty);
  const { isDragActive, acceptsDrop } = useDragContext();
  // A section the dragged task cannot move to takes no drop: the row used to
  // land, get refused by the daemon and snap back without a word.
  const accepts = acceptsDrop(status);

  const { setNodeRef, isOver } = useDroppable({
    id: status,
    disabled: !accepts,
  });

  React.useEffect(() => {
    if (isDragActive) {
      setIsOpen(true);
    } else {
      setIsOpen(!isEmpty);
    }
  }, [isDragActive, isEmpty]);

  return (
    <section className="flex flex-col gap-1 not-first:mt-4">
      <Collapsible open={isOpen} onOpenChange={setIsOpen}>
        <CollapsibleTrigger asChild>
          <header className="group/collapsible-trigger flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-accent/50">
            <CollapsibleIcon>
              <Icon className={`size-4 ${config.color}`} />
            </CollapsibleIcon>
            <CollapsibleTitle className="flex items-center gap-2 normal-case tracking-normal text-sm font-medium text-foreground">
              {config.label}
            </CollapsibleTitle>
            <span className="h-5 px-1.5 text-xs text-muted-foreground">
              {tasks.length}
            </span>
          </header>
        </CollapsibleTrigger>

        <CollapsibleContent>
          <div
            ref={setNodeRef}
            className={cn(
              "flex flex-col rounded-md border shadow-inner bg-muted divide-y overflow-hidden transition-[border-color,background-color,box-shadow]",
              isDragActive && accepts && "border-dashed border-border/85 bg-muted/30",
              isDragActive && !accepts && "opacity-50",
              isOver && "border-primary/35 bg-accent/30 ring-1 ring-primary/10",
            )}
          >
            {isEmpty ? (
              <div
                className={cn(
                  "flex h-11 items-center justify-center px-3 text-xs text-muted-foreground transition-colors",
                  isOver && "text-foreground font-medium",
                )}
              >
                {isDragActive && !accepts
                  ? t("This task cannot move here")
                  : isOver
                    ? t("Drop task here")
                    : t("No tasks in {{status}}", { status: config.label })}
              </div>
            ) : (
              tasks.map((task) => (
                <DraggableTaskListRow key={task.id} task={task} />
              ))
            )}
          </div>
        </CollapsibleContent>
      </Collapsible>
    </section>
  );
});
