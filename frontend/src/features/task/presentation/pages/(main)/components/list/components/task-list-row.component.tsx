import React, { useCallback } from "react";
import { Link, useRouter } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
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
import {
  TASK_PRIORITY_CONFIG,
  TASK_STATUS_CONFIG,
  TASK_STATUS_ORDER,
} from "@/features/task/presentation/consts/task";
import { UserIcon } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { TaskActionsDropdown } from "@/features/task/presentation/components/dropdowns";
import { SetPriorityDropdown } from "@/features/task/presentation/components/dropdowns/set-priority.dropdown";
import { SetAssigneeDropdown } from "@/features/task/presentation/components/dropdowns/set-assignee.dropdown";
import { SetTypeDropdown } from "@/features/task/presentation/components/dropdowns/set-type.dropdown";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

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
  const directory = aos.stores.workspace.useState(
    (state) => state.directory,
  );
  const self = aos.stores.auth.useState((state) => state.user);
  const projects = aos.stores.projects.useState((state) => state.items);
  const currentWorkspace = aos.stores.workspace.useState(
    (state) => state.current,
  );
  const taskType = currentWorkspace?.tasks?.find((t) => t.id === task.type);

  const status = TaskHelper.getStatus(task.status);
  const StatusIcon = status.icon;

  const priority = TASK_PRIORITY_CONFIG[task.priority];
  const PriorityIcon = priority.icon;

  const assigneeView = resolveAssignee(
    { ...directory, self },
    task.assigned,
  );
  const isAgent = assigneeView?.type === "agent";
  const assigneeName = assigneeView?.name || task.assigned || "";

  const project = projects.find((p) => p.id === task.project);

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
        toast.error("Failed to update priority");
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
        toast.error("Failed to update assignee");
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
        toast.error("Failed to update status");
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
        toast.error("Failed to update due date");
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
      toast.error("Failed to delete task");
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
    toast.success("Prompt copied");
  }, [task.id, task.name, task.summary, task.content]);

  const handleOpenWorktree = useCallback(() => {
    toast.message("Open worktree coming soon");
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
        toast.error("Failed to update type");
      }
    },
    [task.id, router],
  );

  const handleProjectChange = useCallback(
    async (project: string | undefined) => {
      try {
        await aos.client.task.update.mutate({
          params: { task: task.id },
          body: { project },
        });
        toast.success(project ? `Project updated` : "Project removed");
        router.invalidate();
      } catch (error) {
        toast.error("Failed to update project");
      }
    },
    [task.id, router],
  );

  return (
    <div
      className={cn(
        "grid min-h-11 w-full items-center gap-2 px-3 py-2 transition-colors hover:border-input hover:bg-accent/40",
        dragHandle
          ? "grid-cols-[auto_auto_auto_auto_1fr_auto_auto_auto_auto]"
          : "grid-cols-[auto_auto_auto_1fr_auto_auto_auto_auto]",
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
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetPriorityDropdown
            currentPriority={task.priority}
            onPriorityChange={handlePriorityChange}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/tasks/$id" params={{ id: task.id }}>
        <span className="max-w-20 truncate font-mono text-sm text-muted-foreground">
          {task.id}
        </span>
      </Link>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center gap-1 rounded p-1 hover:bg-accent">
            <StatusIcon className={`size-3.5 ${status.color}`} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {TASK_STATUS_ORDER.map((s) => {
            const config = TASK_STATUS_CONFIG[s];
            const Icon = config.icon;
            return (
              <DropdownMenuItem
                key={s}
                onClick={() => handleStatusChange(s)}
                className="flex items-center gap-2"
              >
                <Icon className={`size-4 ${config.color}`} />
                <span>{config.label}</span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/tasks/$id" params={{ id: task.id }}>
        <span className="truncate text-sm font-medium">{task.name}</span>
      </Link>

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
            onProjectChange={handleProjectChange}
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
                  name={assigneeName.toLowerCase()}
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
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
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
  );
});
