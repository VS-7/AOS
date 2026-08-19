"use client";

import React from "react";
import { SignalZero, SignalLow, SignalMedium, SignalHigh, FlagIcon } from "lucide-react";
import {
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
} from "@/components/ui/dropdown-menu";
import type { FractalTaskPriority } from "@/features/task/interfaces/task.interfaces";
import { TASK_PRIORITY_CONFIG, TASK_PRIORITY_ORDER } from "@/features/task/presentation/consts/task";

interface SetPriorityDropdownProps {
  currentPriority: FractalTaskPriority;
  onPriorityChange: (priority: FractalTaskPriority) => void;
}

const PRIORITY_ICONS: Record<FractalTaskPriority, React.ElementType> = {
  no_priority: SignalZero,
  low: SignalLow,
  medium: SignalMedium,
  high: SignalHigh,
  urgent: FlagIcon,
};

export function SetPriorityDropdown({ currentPriority, onPriorityChange }: SetPriorityDropdownProps) {
  return (
    <DropdownMenuRadioGroup
      value={currentPriority}
      onValueChange={(value) => onPriorityChange(value as FractalTaskPriority)}
    >
      {TASK_PRIORITY_ORDER.map((priority) => {
        const config = TASK_PRIORITY_CONFIG[priority];
        const Icon = PRIORITY_ICONS[priority];

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
