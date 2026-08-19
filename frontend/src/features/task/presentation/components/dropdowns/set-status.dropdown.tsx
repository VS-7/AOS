import React from "react";
import {
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { TASK_STATUS_CONFIG, TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";

interface SetStatusDropdownProps {
  currentStatus: string;
  onStatusChange: (status: string) => void;
}

export function SetStatusDropdown({ currentStatus, onStatusChange }: SetStatusDropdownProps) {
  return (
    <div className="flex flex-col gap-1">
      {TASK_STATUS_ORDER.map((s) => {
        const config = TASK_STATUS_CONFIG[s];
        const Icon = config.icon;
        return (
          <DropdownMenuItem
            key={s}
            onClick={() => onStatusChange(s)}
            className="flex items-center gap-2"
          >
            <Icon className={`size-4 ${config.color}`} />
            <span>{config.label}</span>
            {currentStatus === s && (
              <span className="ml-auto text-xs text-muted-foreground">✓</span>
            )}
          </DropdownMenuItem>
        );
      })}
    </div>
  );
}
