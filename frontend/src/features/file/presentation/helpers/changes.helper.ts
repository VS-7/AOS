import type { FileChangeEntry } from "@/features/file/interfaces/file.interfaces";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  formatExplorerContextLabel,
  parseExplorerContext,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import { t } from "@/lib/i18n";

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
): FileExplorerContext {
  return parseExplorerContext(metadata?.fileExplorerContext);
}

export function formatChangesContextScope(
  context: FileExplorerContext,
): string {
  if (context.type === "task") return t("Task");
  return t("Local");
}

export function formatChangesContextRef(
  context: FileExplorerContext,
): string {
  if (context.type === "main") return "main";
  if (context.type === "branch") return context.branch;
  return formatExplorerContextLabel(context);
}

export function formatChangesCountLabel(params: {
  fileCount: number;
  readOnly: boolean;
}): string {
  // Whole sentences per case rather than words glued together: the order of
  // count, kind and noun is not the same in every language.
  const values = { count: params.fileCount };
  if (params.readOnly) {
    return params.fileCount === 1
      ? t("{{count}} Branch Change", values)
      : t("{{count}} Branch Changes", values);
  }
  return params.fileCount === 1
    ? t("{{count}} Uncommitted Change", values)
    : t("{{count}} Uncommitted Changes", values);
}

export function formatChangeStatusLabel(
  status: FileChangeEntry["status"],
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
  status: FileChangeEntry["status"],
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
