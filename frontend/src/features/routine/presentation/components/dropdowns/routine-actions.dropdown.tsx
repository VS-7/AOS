"use client";

import {
  CopyIcon,
  MoreHorizontalIcon,
  PlayIcon,
  TrashIcon,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";

interface RoutineActionsDropdownProps {
  routine: Routine;
  onFire?: () => void;
  onCopyIdentifier?: () => void;
  onDelete?: () => void;
  isFiring?: boolean;
}

export function RoutineActionsDropdown({
  routine,
  onFire,
  onCopyIdentifier,
  onDelete,
  isFiring,
}: RoutineActionsDropdownProps) {
  const isDisabled = routine.status === "disabled";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex items-center justify-center rounded p-1 hover:bg-accent"
          aria-label={`Actions for ${routine.name}`}
        >
          <MoreHorizontalIcon className="size-4 text-muted-foreground hover:text-foreground" />
        </button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-52">
        {onFire && (
          <DropdownMenuItem
            onClick={onFire}
            disabled={isDisabled || isFiring}
            className="flex items-center gap-2"
          >
            <PlayIcon className="size-4" />
            <span>{isFiring ? "Starting…" : "Fire now"}</span>
          </DropdownMenuItem>
        )}

        {onCopyIdentifier && (
          <DropdownMenuItem
            onClick={onCopyIdentifier}
            className="flex items-center gap-2"
          >
            <CopyIcon className="size-4" />
            <span>Copy identifier</span>
          </DropdownMenuItem>
        )}

        {(onFire || onCopyIdentifier) && onDelete && <DropdownMenuSeparator />}

        {onDelete && (
          <DropdownMenuItem
            onClick={onDelete}
            variant="destructive"
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
