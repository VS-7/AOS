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
import { allowedMoves } from "@/features/task/presentation/helpers/task-lifecycle.helper";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { UserIcon } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns";
import { SetPriorityDropdown } from "@/features/task/presentation/components/dropdowns/set-priority.dropdown";
import { SetAssigneeDropdown } from "@/features/task/presentation/components/dropdowns/set-assignee.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import { SetStatusDropdown } from "@/features/task/presentation/components/dropdowns/set-status.dropdown";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { useTaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { useAssigneeDirectory } from "@/features/task/presentation/hooks/assignee-directory.hook";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface TaskListRowProps {
  task: Task;
  isDragOverlay?: boolean;
  isDragging?: boolean;
  dragHandle?: React.ReactNode;
}

export const TaskListRow = React.memo(function TaskListRow({
  task,
  isDragOverlay,
  isDragging,
  dragHandle,
}: TaskListRowProps) {
  const router = useRouter();
  const directory = useAssigneeDirectory();
  const projects = aos.stores.projects.useState((state) => state.items);
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );
  const taskType = currentWorkspace?.tasks?.find((t) => t.id === task.type);
  const actions = useTaskActions(task, { onChanged: () => void router.invalidate() });

  const status = TaskHelper.getStatus(task.status);
  const StatusIcon = status.icon;

  const priority = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priority.icon;

  const assigneeView = resolveTaskAssignee(directory, task);
  const isAgent = assigneeView?.type === "agent";

  const project = projects.find((p) => p.id === task.project);

  return (
    <div
      className={cn(
        "grid min-h-11 w-full items-center gap-2 px-3 py-2 transition-colors hover:border-input hover:bg-accent/40",
        dragHandle
          ? "grid-cols-[auto_auto_auto_auto_minmax(0,1fr)_auto_auto_auto_auto]"
          : "grid-cols-[auto_auto_auto_minmax(0,1fr)_auto_auto_auto_auto]",
        isDragging && "opacity-40 bg-muted/50",
        isDragOverlay &&
          "rotate-1 scale-[1.01] shadow-xl ring-1 ring-primary/20 bg-background border",
      )}
    >
      {dragHandle}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center justify-center rounded p-1 hover:bg-accent">
            <PriorityIcon className={`size-3.5 ${priority.colorClass}`} />
            <span className="sr-only">{t("Priority")}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetPriorityDropdown
            currentPriority={task.priority}
            onPriorityChange={actions.setPriority}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <Link
        to="/tasks/$id"
        params={{ id: task.id }}
        title={task.id}
        className="font-mono text-xs text-muted-foreground"
      >
        {TaskHelper.shortId(task.id)}
      </Link>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center gap-1 rounded p-1 hover:bg-accent">
            <StatusIcon className={`size-3.5 ${status.color}`} />
            <span className="sr-only">{status.label}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetStatusDropdown
            currentStatus={task.status}
            allowed={allowedMoves(task)}
            onStatusChange={actions.setStatus}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/tasks/$id" params={{ id: task.id }} className="block min-w-0">
        <span className="block truncate text-sm font-medium">{task.name}</span>
      </Link>

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
          <Badge variant="outline" className="min-w-24 max-w-28 cursor-pointer">
            <Icon
              value={ProjectHelper.getIcon(project?.icon)}
              fallback="Folder"
              className="size-3 shrink-0 text-muted-foreground"
            />
            <span className="truncate text-xs">
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

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center justify-center rounded p-1 hover:bg-accent">
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
                <AvatarFallback>
                  <UserIcon className="size-3 text-muted-foreground" />
                </AvatarFallback>
              </Avatar>
            )}
            <span className="sr-only">{assigneeView?.name || t("Unassigned")}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
          <SetAssigneeDropdown
            currentAssignee={task.assigned}
            onAssigneeChange={actions.setAssignee}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <TaskActionsDropdown task={task} actions={actions} />
    </div>
  );
});
