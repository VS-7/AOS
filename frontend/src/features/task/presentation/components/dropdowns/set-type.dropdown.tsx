import React from "react";
import {
  DropdownMenuItem,
  DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

interface SetTypeDropdownProps {
  currentType?: string;
  onTypeChange: (type: string) => void;
}

export function SetTypeDropdown({ currentType, onTypeChange }: SetTypeDropdownProps) {
  // The source read this off `aos.useContext().workspaces` — AOS's
  // global route context, populated by a `withContext(...)` this vertical
  // slice's `aos.tsx` doesn't wire (route context is empty here). Every
  // other place `task` reads the workspace's task-type taxonomy
  // (`filter.tsx`, the kanban/list cards) already does it through
  // `aos.stores.workspace`'s `current.tasks`, so this follows that same,
  // already-established path instead of adding a second mechanism for one
  // dropdown.
  const taskTypes = aos.stores.workspace.useState((state) => state.current?.tasks) || [];

  if (taskTypes.length === 0) {
    return (
      <DropdownMenuItem className="text-muted-foreground">
        {t("No task types available")}
      </DropdownMenuItem>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        {t("Task Type")}
      </DropdownMenuLabel>
      {taskTypes.map((taskType) => (
        <DropdownMenuItem
          key={taskType.id}
          onClick={() => onTypeChange(taskType.id)}
          className="flex items-center gap-2"
        >
          <span
            className="size-2 rounded-full"
            style={{ backgroundColor: taskType.color }}
          />
          <span>{taskType.label}</span>
          {currentType === taskType.id && (
            <span className="ml-auto text-xs text-muted-foreground">✓</span>
          )}
        </DropdownMenuItem>
      ))}
    </div>
  );
}
