"use client";

import { CheckIcon, Target } from "lucide-react";

import { aos } from "@/app/aos";
import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "./dropdown-menu";

interface GoalSelectorDropdownProps {
  currentGoal?: string;
  onGoalChange: (goal: string | undefined) => void;
}

export function GoalSelectorDropdown({
  currentGoal,
  onGoalChange,
}: GoalSelectorDropdownProps) {
  const goals = aos.stores.goals.useState((state) => state.items);

  if (goals.length === 0) {
    return (
      <DropdownMenuItem className="text-muted-foreground">
        No goals available
      </DropdownMenuItem>
    );
  }

  return (
    <div className="flex flex-col gap-1 p-2">
      <DropdownMenuItem
        onClick={() => onGoalChange(undefined)}
        className="flex items-center gap-2"
      >
        <Target className="size-4 text-muted-foreground shrink-0" />
        <span>No goal</span>
        {!currentGoal ? (
          <CheckIcon className="ml-auto size-4 text-muted-foreground shrink-0" />
        ) : null}
      </DropdownMenuItem>

      <DropdownMenuSeparator />

      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">
        Goals
      </DropdownMenuLabel>

      {goals.map((goal) => (
        <DropdownMenuItem
          key={goal.id}
          onClick={() => onGoalChange(goal.id)}
          className="flex items-center gap-2"
        >
          <Target className="size-4 text-muted-foreground shrink-0" />
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-xs text-muted-foreground shrink-0">
              {goal.id}
            </span>
            <span className="text-sm line-clamp-1">{goal.title}</span>
          </div>
          {currentGoal === goal.id ? (
            <CheckIcon className="ml-auto size-4 text-muted-foreground shrink-0" />
          ) : null}
        </DropdownMenuItem>
      ))}
    </div>
  );
}
