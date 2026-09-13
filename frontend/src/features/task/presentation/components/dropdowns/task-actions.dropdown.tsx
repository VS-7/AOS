import React from "react";
import {
  MoreHorizontalIcon,
  CalendarIcon,
  TagIcon,
  TrashIcon,
  CopyIcon,
  CopyPlusIcon,
  GitBranchIcon,
  CircleDotIcon,
  FlagIcon,
  MessageSquareIcon,
  UserIcon,
} from "lucide-react";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SetPriorityDropdown } from "./set-priority.dropdown";
import { SetAssigneeDropdown } from "./set-assignee.dropdown";
import { SetTypeDropdown } from "./set-type.dropdown";
import { SetStatusDropdown } from "./set-status.dropdown";
import { TaskDueDateCalendar } from "@/features/task/presentation/components/due-date/task-due-date";
import { allowedMoves } from "@/features/task/presentation/helpers/task-lifecycle.helper";
import type { TaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { t } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface TaskActionsDropdownProps {
  task: Task;
  actions: TaskActions;
  className?: string;
}

export function TaskActionsDropdown({ task, actions, className }: TaskActionsDropdownProps) {
  // Controlled so picking a day closes the menu: the calendar is not a menu
  // item, and the menu otherwise stayed open over the page after the change.
  const [open, setOpen] = React.useState(false);

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      {/* A button, not a bare icon: the icon was not focusable, so the menu
          could not be reached from the keyboard at all. */}
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn("size-7 shrink-0 text-muted-foreground hover:text-foreground", className)}
        >
          <MoreHorizontalIcon className="size-4" />
          <span className="sr-only">{t("Task actions")}</span>
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <FlagIcon className="size-4" />
              <span>{t("Priority")}</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <SetPriorityDropdown currentPriority={task.priority} onPriorityChange={actions.setPriority} />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <UserIcon className="size-4" />
              <span>{t("Assignee")}</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="w-64">
            <SetAssigneeDropdown currentAssignee={task.assigned} onAssigneeChange={actions.setAssignee} />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <TagIcon className="size-4" />
              <span>{t("Type")}</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <SetTypeDropdown currentType={task.type} onTypeChange={actions.setType} />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <CircleDotIcon className="size-4" />
              <span>{t("Status")}</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <SetStatusDropdown
              currentStatus={task.status}
              allowed={allowedMoves(task)}
              onStatusChange={actions.setStatus}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <CalendarIcon className="size-4" />
              <span>{task.dueAt ? t("Change due date") : t("Set due date")}</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="p-0">
            <TaskDueDateCalendar
              value={task.dueAt}
              onChange={(dueAt) => {
                setOpen(false);
                void actions.setDueDate(dueAt);
              }}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        <DropdownMenuSeparator />

        {task.chat && (
          <DropdownMenuItem
            onClick={() => openChatTab({ chatId: task.chat!, title: task.name })}
            inset
            className="flex items-center gap-2"
          >
            <MessageSquareIcon className="size-4" />
            <span>{t("Open chat")}</span>
          </DropdownMenuItem>
        )}

        {/* "Open worktree" only ever toasted "coming soon". What the daemon
            does offer is cutting the checkout (tasks_branch), and once it
            exists its path is what a person needs. */}
        {task.worktree?.path ? (
          <DropdownMenuItem
            onClick={() => void actions.copyPath(task.worktree.path!)}
            inset
            className="flex items-center gap-2"
          >
            <GitBranchIcon className="size-4" />
            <span>{t("Copy worktree path")}</span>
          </DropdownMenuItem>
        ) : task.worktree?.enabled ? (
          <DropdownMenuItem
            onClick={() => void actions.createWorktree()}
            inset
            className="flex items-center gap-2"
          >
            <GitBranchIcon className="size-4" />
            <span>{t("Create worktree")}</span>
          </DropdownMenuItem>
        ) : null}

        <DropdownMenuItem onClick={() => void actions.copyPrompt()} inset className="flex items-center gap-2">
          <CopyPlusIcon className="size-4" />
          <span>{t("Copy as prompt")}</span>
        </DropdownMenuItem>

        <DropdownMenuItem onClick={() => void actions.copyIdentifier()} inset className="flex items-center gap-2">
          <CopyIcon className="size-4" />
          <span>{t("Copy task ID")}</span>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          onClick={() => void actions.remove()}
          variant="destructive"
          inset
          className="flex items-center gap-2"
        >
          <TrashIcon className="size-4" />
          <span>{t("Delete")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
