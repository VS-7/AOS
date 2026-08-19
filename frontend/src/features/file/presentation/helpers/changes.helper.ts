import type { FractalFileChangeEntry } from "@/features/file/interfaces/file.interfaces";
import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  formatExplorerContextLabel,
  parseExplorerContext,
} from "@/features/file/presentation/helpers/files-explorer.helper";

export type ChangesDiffStyle = "unified" | "split";

export interface ChangesPanelPreferences {
  diffStyle: ChangesDiffStyle;
  ignoreWhitespace: boolean;
  wordWrap: boolean;
}

export const DEFAULT_CHANGES_PREFERENCES: ChangesPanelPreferences = {
  diffStyle: "unified",
  ignoreWhitespace: false,
  wordWrap: false,
};

export function getChangesTabContext(
  metadata?: Record<string, string | number | boolean>,
): FractalFileExplorerContext {
  return parseExplorerContext(metadata?.fileExplorerContext);
}

export function formatChangesContextScope(
  context: FractalFileExplorerContext,
): string {
  if (context.type === "task") return "Task";
  return "Local";
}

export function formatChangesContextRef(
  context: FractalFileExplorerContext,
): string {
  if (context.type === "main") return "main";
  if (context.type === "branch") return context.branch;
  return formatExplorerContextLabel(context);
}

export function formatChangesCountLabel(params: {
  fileCount: number;
  readOnly: boolean;
}): string {
  const noun = params.fileCount === 1 ? "Change" : "Changes";
  const kind = params.readOnly ? "Branch" : "Uncommitted";
  return `${params.fileCount} ${kind} ${noun}`;
}

export function formatChangeStatusLabel(
  status: FractalFileChangeEntry["status"],
): string {
  switch (status) {
    case "added":
      return "A";
    case "modified":
      return "M";
    case "deleted":
      return "D";
    case "renamed":
      return "R";
    case "untracked":
      return "U";
    default:
      return "?";
  }
}

export function changeStatusClassName(
  status: FractalFileChangeEntry["status"],
): string {
  switch (status) {
    case "added":
    case "untracked":
      return "text-emerald-600 dark:text-emerald-400";
    case "modified":
    case "renamed":
      return "text-sky-600 dark:text-sky-400";
    case "deleted":
      return "text-destructive";
    default:
      return "text-muted-foreground";
  }
}
