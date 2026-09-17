"use client";

import React from "react";
import {
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
} from "@/components/ui/dropdown-menu";
import type { TaskPriority } from "@/features/task/interfaces/task.interfaces";
import { TASK_PRIORITY_CONFIG, TASK_PRIORITY_ORDER } from "@/features/task/presentation/consts/task";

interface SetPriorityDropdownProps {
  currentPriority: TaskPriority;
  onPriorityChange: (priority: TaskPriority) => void;
}

export function SetPriorityDropdown({ currentPriority, onPriorityChange }: SetPriorityDropdownProps) {
  return (
    <DropdownMenuRadioGroup
      value={currentPriority}
      onValueChange={(value) => onPriorityChange(value as TaskPriority)}
    >
      {TASK_PRIORITY_ORDER.map((priority) => {
        const config = TASK_PRIORITY_CONFIG[priority];
        const Icon = config.icon;

        return (
          <DropdownMenuRadioItem
            key={priority}
            value={priority}
            className="flex items-center gap-2"
          >
            <Icon className={`size-4 ${config.colorClass}`} />
            <span className="whitespace-nowrap pr-2">{config.label}</span>
          </DropdownMenuRadioItem>
        );
      })}
    </DropdownMenuRadioGroup>
  );
}
