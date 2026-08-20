"use client";

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
} from "lucide-react";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SetPriorityDropdown } from "./set-priority.dropdown";
import { SetAssigneeDropdown } from "./set-assignee.dropdown";
import { SetTypeDropdown } from "./set-type.dropdown";
import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import { TASK_STATUS_CONFIG, TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";

interface TaskActionsDropdownProps {
  task: Task;
  onPriorityChange: (priority: TaskPriority) => void;
  onAssigneeChange: (assignee: string | undefined) => void;
  onTypeChange?: (type: string) => void;
  onStatusChange?: (status: Task["status"]) => void;
  onDueDateChange?: (dueAt: string | undefined) => void;
  onDelete?: () => void;
  onCopyIdentifier?: () => void;
  onCopyPrompt?: () => void;
  onOpenWorktree?: () => void;
  onOpenChat?: () => void;
}

export function TaskActionsDropdown({
  task,
  onPriorityChange,
  onAssigneeChange,
  onTypeChange,
  onStatusChange,
  onDueDateChange,
  onDelete,
  onCopyIdentifier,
  onCopyPrompt,
  onOpenWorktree,
  onOpenChat,
}: TaskActionsDropdownProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <MoreHorizontalIcon className="size-4 cursor-pointer text-muted-foreground hover:text-foreground" />
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-64">
        {/* Priority Submenu */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <FlagIcon className="size-4" />
              <span>Priority</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <SetPriorityDropdown
              currentPriority={task.priority}
              onPriorityChange={onPriorityChange}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Assignee Submenu */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger inset>
            <span className="flex items-center gap-2">
              <TagIcon className="size-4" />
              <span>Assignee</span>
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <SetAssigneeDropdown
              currentAssignee={task.assigned}
              onAssigneeChange={onAssigneeChange}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Type Submenu */}
        {onTypeChange && (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger inset>
              <span className="flex items-center gap-2">
                <TagIcon className="size-4" />
                <span>Type</span>
              </span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <SetTypeDropdown
                currentType={task.type}
                onTypeChange={onTypeChange}
              />
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}

        {/* Status Submenu */}
        {onStatusChange && (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger inset>
              <span className="flex items-center gap-2">
                <CircleDotIcon className="size-4" />
                <span>Status</span>
              </span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              {TASK_STATUS_ORDER.map((status) => {
                const config = TASK_STATUS_CONFIG[status];
                const StatusIcon = config.icon;
                return (
                  <DropdownMenuItem
                    key={status}
                    onClick={() => onStatusChange(status)}
                    className="flex items-center gap-2"
                  >
                    <StatusIcon className={`size-4 ${config.color}`} />
                    <span>{config.label}</span>
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}

        {/* Due Date */}
        {onDueDateChange && (
          <DropdownMenuItem onClick={() => {
            // In a real implementation, this would open a date picker
            const date = prompt("Enter due date (YYYY-MM-DD):");
            if (date) {
              onDueDateChange(date);
            }
          }} inset className="flex items-center gap-2">
            <CalendarIcon className="size-4" />
            <span>Set due date</span>
          </DropdownMenuItem>
        )}

        <DropdownMenuSeparator />

        {/* Open Chat */}
        {task.chat && (
          <DropdownMenuItem
            onClick={() => {
              if (onOpenChat) {
                onOpenChat();
              } else {
                openChatTab({ chatId: task.chat!, title: task.name });
              }
            }}
            inset
            className="flex items-center gap-2"
          >
            <MessageSquareIcon className="size-4" />
            <span>Open chat</span>
          </DropdownMenuItem>
        )}

        {/* Worktree */}
        {onOpenWorktree && (
          <DropdownMenuItem onClick={onOpenWorktree} inset className="flex items-center gap-2">
            <GitBranchIcon className="size-4" />
            <span>Open worktree</span>
          </DropdownMenuItem>
        )}

        {/* Copy Prompt */}
        {onCopyPrompt && (
          <DropdownMenuItem onClick={onCopyPrompt} inset className="flex items-center gap-2">
            <CopyPlusIcon className="size-4" />
            <span>Copy as prompt</span>
            <DropdownMenuShortcut>⌘⇧P</DropdownMenuShortcut>
          </DropdownMenuItem>
        )}

        {/* Copy Identifier */}
        {onCopyIdentifier && (
          <DropdownMenuItem onClick={onCopyIdentifier} inset className="flex items-center gap-2">
            <CopyIcon className="size-4" />
            <span>Copy identifier</span>
          </DropdownMenuItem>
        )}



        <DropdownMenuSeparator />

        {/* Delete */}
        {onDelete && (
          <DropdownMenuItem
            onClick={onDelete}
            variant="destructive"
            inset
            className="flex items-center gap-2"
          >
            <TrashIcon className="size-4" />
            <span>Delete</span>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
