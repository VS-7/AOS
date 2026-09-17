import React from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Avatar,
  AvatarFallback,
  AvatarAgentFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { assigneeInitials, resolveTaskAssignee } from "@/features/task/presentation/helpers/assignee.helper";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns";
import { SetAssigneeDropdown } from "@/features/task/presentation/components/dropdowns/set-assignee.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import { useTaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { useAssigneeDirectory } from "@/features/task/presentation/hooks/assignee-directory.hook";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

import { useDraggable } from "@dnd-kit/core";
import { GripVertical } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";

interface TaskKanbanCardProps {
  task: Task;
  isDragging: boolean;
  isDragOverlay?: boolean;
}

export const TaskKanbanCard = React.memo(function TaskKanbanCard({
  task,
  isDragging,
  isDragOverlay,
}: TaskKanbanCardProps) {
  const { attributes, listeners, setNodeRef } = useDraggable({
    id: task.id,
    disabled: isDragOverlay,
  });
  const router = useRouter();
  const directory = useAssigneeDirectory();
  const projects = aos.stores.projects.useState((state) => state.items);
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );
  const taskType = currentWorkspace?.tasks?.find((t) => t.id === task.type);
  const project = projects.find((p) => p.id === task.project);
  const actions = useTaskActions(task, { onChanged: () => void router.invalidate() });

  const status = TaskHelper.getStatus(task.status);
  const StatusIcon = status.icon;
  const priority = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priority.icon;

  const assigneeView = resolveTaskAssignee(directory, task);
  const isAgent = assigneeView?.type === "agent";

  return (
    <div
      ref={setNodeRef}
      className={cn(
        "group flex h-40 shrink-0 flex-col gap-3 rounded-xl border bg-card px-4 py-3 shadow-sm transition-[border-color,background-color,opacity,box-shadow] hover:border-input hover:bg-accent/20",
        isDragging &&
          "scale-[0.985] border-primary/35 bg-accent/30 opacity-55 shadow-none ring-1 ring-primary/10",
        isDragOverlay &&
          "rotate-1 scale-[1.01] shadow-xl ring-1 ring-primary/20 bg-background border",
      )}
    >
      <div className="flex items-center gap-2">
        <div
          {...(!isDragOverlay ? { ...attributes, ...listeners } : {})}
          className={cn(
            "flex items-center justify-center p-1 text-muted-foreground/30",
            !isDragOverlay
              ? "cursor-grab hover:text-muted-foreground/80 active:cursor-grabbing"
              : "cursor-default",
          )}
        >
          <GripVertical className="size-3.5" />
        </div>
        <StatusIcon className={`size-4 shrink-0 ${status.color}`} />
        <span className="font-mono text-xs text-muted-foreground" title={task.id}>
          {TaskHelper.shortId(task.id)}
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="ml-auto flex items-center justify-center rounded p-1 hover:bg-accent">
              {isAgent ? (
                <Avatar size="sm">
                  <AvatarAgentFallback
                    size={26}
                    name={assigneeView.name.toLowerCase()}
                  />
                </Avatar>
              ) : assigneeView ? (
                <Avatar size="sm">
                  {assigneeView.image ? (
                    <AvatarImage src={assigneeView.image} alt={assigneeView.name} />
                  ) : (
                    <AvatarFallback>{assigneeInitials(assigneeView.name)}</AvatarFallback>
                  )}
                </Avatar>
              ) : (
                <Avatar size="sm">
                  <AvatarFallback className="bg-muted">
                    <span className="text-xs text-muted-foreground">--</span>
                  </AvatarFallback>
                </Avatar>
              )}
              <span className="sr-only">{assigneeView?.name || t("Unassigned")}</span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            <SetAssigneeDropdown
              currentAssignee={task.assigned}
              onAssigneeChange={actions.setAssignee}
            />
          </DropdownMenuContent>
        </DropdownMenu>
        <TaskActionsDropdown task={task} actions={actions} />
      </div>

      <Link
        to="/tasks/$id"
        params={{ id: task.id }}
        className="flex min-h-0 flex-1 items-start"
      >
        <p className="line-clamp-2 text-sm font-medium leading-5">
          {task.name}
        </p>
      </Link>

      <div className="mt-auto flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Badge variant="outline" className="gap-1">
            <PriorityIcon className={`size-3 ${priority.colorClass}`} />
            {priority.label}
          </Badge>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Badge
                variant="outline"
                className="cursor-pointer gap-1"
              >
                <div className="block size-2 border rounded-full" style={{ borderColor: taskType?.color }} />
                {taskType?.label || task.type || t("No type")}
              </Badge>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <SetTypeDropdown
                currentType={task.type}
                onTypeChange={actions.setType}
              />
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Badge
                variant="outline"
                className="max-w-28 cursor-pointer gap-1"
              >
                <Icon
                  value={ProjectHelper.getIcon(project?.icon)}
                  fallback="Folder"
                  className="size-3 shrink-0 text-muted-foreground"
                />
                <span className="truncate">
                  {project?.name || task.project || "—"}
                </span>
              </Badge>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-72">
              <ProjectSelectorDropdown
                currentProject={task.project}
                onProjectChange={actions.setProject}
              />
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  );
});
