import React from "react";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { TASK_STATUS_CONFIG, TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import type { TaskStatus } from "@/features/task/interfaces/task.interfaces";

interface SetStatusDropdownProps {
  currentStatus: TaskStatus;
  onStatusChange: (status: TaskStatus) => void;
  /** The statuses listed. Every one, in lifecycle order, unless a caller narrows it. */
  statuses?: readonly TaskStatus[];
  /**
   * The statuses that can be picked. The rest of the list is shown disabled,
   * so the lifecycle stays readable: every picker used to offer all eight
   * and let the daemon refuse most of them after the click.
   */
  allowed?: readonly TaskStatus[];
}

export function SetStatusDropdown({
  currentStatus,
  onStatusChange,
  statuses = TASK_STATUS_ORDER,
  allowed,
}: SetStatusDropdownProps) {
  return (
    <div className="flex flex-col gap-1">
      {statuses.map((s) => {
        const config = TASK_STATUS_CONFIG[s];
        const Icon = config.icon;
        const current = currentStatus === s;
        const enabled = !allowed || allowed.includes(s);
        return (
          <DropdownMenuItem
            key={s}
            disabled={!current && !enabled}
            onClick={() => {
              if (!current) onStatusChange(s);
            }}
            className="flex items-center gap-2"
          >
            <Icon className={`size-4 ${config.color}`} />
            <span>{config.label}</span>
            {current && <span className="ml-auto text-xs text-muted-foreground">✓</span>}
          </DropdownMenuItem>
        );
      })}
    </div>
  );
}
