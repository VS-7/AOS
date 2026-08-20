import * as React from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import type { FileChangeEntry } from "@/features/file/interfaces/file.interfaces";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  changeStatusClassName,
  formatChangeStatusLabel,
  type ChangesPanelPreferences,
} from "@/features/file/presentation/helpers/changes.helper";
import { ChangesFileDiff } from "./changes-file-diff";

interface ChangesFileItemProps {
  file: FileChangeEntry;
  explorerContext: FileExplorerContext;
  preferences: ChangesPanelPreferences;
  themeType: "light" | "dark";
  expanded: boolean;
  onToggle: () => void;
}

export function ChangesFileItem({
  file,
  explorerContext,
  preferences,
  themeType,
  expanded,
  onToggle,
}: ChangesFileItemProps) {
  return (
    <div className="border-b last:border-b-0">
      <button
        type="button"
        onClick={onToggle}
        className={cn(
          "flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm",
          "hover:bg-muted/50",
          expanded && "bg-muted/30",
        )}
      >
        {expanded ? (
          <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />
        )}

        <span
          className={cn(
            "w-4 shrink-0 text-center text-xs font-semibold",
            changeStatusClassName(file.status),
          )}
        >
          {formatChangeStatusLabel(file.status)}
        </span>

        <span className="min-w-0 flex-1 truncate font-medium">{file.path}</span>

        {file.oldPath ? (
          <span className="hidden max-w-[30%] truncate text-xs text-muted-foreground sm:inline">
            from {file.oldPath}
          </span>
        ) : null}

        <span className="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
          {file.isBinary ? (
            "binary"
          ) : (
            <>
              {(file.additions ?? 0) > 0 ? (
                <span className="text-emerald-600 dark:text-emerald-400">
                  +{(file.additions ?? 0)}
                </span>
              ) : null}
              {(file.additions ?? 0) > 0 && (file.deletions ?? 0) > 0 ? " " : null}
              {(file.deletions ?? 0) > 0 ? (
                <span className="text-destructive">−{(file.deletions ?? 0)}</span>
              ) : null}
              {(file.additions ?? 0) === 0 && (file.deletions ?? 0) === 0 ? "—" : null}
            </>
          )}
        </span>
      </button>

      {expanded ? (
        <ChangesFileDiff
          path={file.path}
          explorerContext={explorerContext}
          preferences={preferences}
          themeType={themeType}
        />
      ) : null}
    </div>
  );
}
