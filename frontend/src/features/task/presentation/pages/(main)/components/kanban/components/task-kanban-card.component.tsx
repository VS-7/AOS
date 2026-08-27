import React, { useCallback } from "react";
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
import { assigneeInitials, resolveAssignee } from "@/features/task/presentation/helpers/assignee.helper";
import { TASK_PRIORITY_CONFIG } from "@/features/task/presentation/consts/task";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns";
import { SetAssigneeDropdown } from "@/features/task/presentation/components/dropdowns/set-assignee.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

import { useDraggable } from "@dnd-kit/core";
import { GripVertical } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { t } from "@/lib/i18n";

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
  const directory = aos.stores.workspace.useState(
    (state) => state.directory,
  );
  const self = aos.stores.auth.useState((state) => state.user);
  const projects = aos.stores.projects.useState((state) => state.items);
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );
  const taskType = currentWorkspace?.tasks?.find((t) => t.id === task.type);
  const project = projects.find((p) => p.id === task.project);

  const status = TaskHelper.getStatus(task.status);
  const StatusIcon = status.icon;
  const priority = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priority.icon;

  const assigneeView = resolveAssignee(
    { ...directory, self },
    task.assigned,
  );
  const isAgent = assigneeView?.type === "agent";

  const handlePriorityChange = useCallback(
    async (priority: Task["priority"]) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { priority },
        });
        toast.success(
          `Priority updated to ${TASK_PRIORITY_CONFIG[priority].label}`,
        );
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update priority"));
      }
    },
    [task.id, router],
  );

  const handleAssigneeChange = useCallback(
    async (assignee: string | undefined) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { assigned: assignee },
        });
        const assignedView = resolveAssignee(
          { ...directory, self },
          assignee,
        );
        toast.success(
          assignee
            ? `Assigned to ${assignedView?.name || assignee}`
            : "Unassigned",
        );
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update assignee"));
      }
    },
    [task.id, router, directory, self],
  );

  const handleStatusChange = useCallback(
    async (status: Task["status"]) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { status },
        });
        toast.success(`Moved to ${TaskHelper.getStatus(status).label}`);
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update status"));
      }
    },
    [task.id, router],
  );

  const handleDueDateChange = useCallback(
    async (dueAt: string | undefined) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { dueAt },
        });
        toast.success(dueAt ? `Due date set` : "Due date removed");
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update due date"));
      }
    },
    [task.id, router],
  );

  const handleDelete = useCallback(async () => {
    try {
      await aos.client.task.delete.mutate({ params: { task: task.id } });
      toast.success(`Task ${task.id} deleted`);
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to delete task"));
    }
  }, [task.id, router]);

  const handleCopyIdentifier = useCallback(() => {
    navigator.clipboard.writeText(task.id);
    toast.success(`${task.id} copied`);
  }, [task.id]);

  const handleCopyPrompt = useCallback(() => {
    const promptText = [
      `Task ${task.id}: ${task.name}`,
      task.summary ? `Summary: ${task.summary}` : undefined,
      task.content ? `Content:\n${task.content}` : undefined,
    ]
      .filter(Boolean)
      .join("\n\n");
    navigator.clipboard.writeText(promptText);
    toast.success(t("Prompt copied"));
  }, [task.id, task.name, task.summary, task.content]);

  const handleOpenWorktree = useCallback(() => {
    toast.message(t("Open worktree coming soon"));
  }, []);

  const handleTypeChange = useCallback(
    async (type: string) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { type },
        });
        toast.success(`Type updated`);
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update type"));
      }
    },
    [task.id, router],
  );

  const handleProjectChange = useCallback(
    async (projectId: string | undefined) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { project: projectId },
        });
        toast.success(projectId ? `Project updated` : "Project removed");
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update project"));
      }
    },
    [task.id, router],
  );

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
        <span className="font-mono text-xs text-muted-foreground">
          {task.id}
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="ml-auto flex items-center justify-center rounded p-1 hover:bg-accent">
              {isAgent ? (
                <Avatar size="sm">
                  <AvatarAgentFallback
                    size={26}
                    name={(assigneeView?.name || "").toLowerCase()}
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
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <SetAssigneeDropdown
              currentAssignee={task.assigned}
              onAssigneeChange={handleAssigneeChange}
            />
          </DropdownMenuContent>
        </DropdownMenu>
        <TaskActionsDropdown
          task={task}
          onPriorityChange={handlePriorityChange}
          onAssigneeChange={handleAssigneeChange}
          onTypeChange={handleTypeChange}
          onStatusChange={handleStatusChange}
          onDueDateChange={handleDueDateChange}
          onDelete={handleDelete}
          onCopyIdentifier={handleCopyIdentifier}
          onCopyPrompt={handleCopyPrompt}
          onOpenWorktree={handleOpenWorktree}
        />
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
                className="cursor-pointer capitalize gap-1"
              >
                <div className="block size-2 border rounded-full" style={{ borderColor: taskType?.color }} />
                {task.type}
              </Badge>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <SetTypeDropdown
                currentType={task.type}
                onTypeChange={handleTypeChange}
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
                onProjectChange={handleProjectChange}
              />
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  );
});
