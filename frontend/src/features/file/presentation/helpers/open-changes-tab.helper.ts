import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  formatExplorerContextLabel,
  parseExplorerContext,
  serializeExplorerContext,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import { aos } from "@/app/aos";

function getTabExplorerContext(tab: {
  metadata?: Record<string, string | number | boolean>;
}): FractalFileExplorerContext {
  return parseExplorerContext(tab.metadata?.fileExplorerContext);
}

function contextsMatch(
  left: FractalFileExplorerContext,
  right: FractalFileExplorerContext,
): boolean {
  if (left.type !== right.type) return false;
  if (left.type === "task" && right.type === "task") {
    return left.taskId === right.taskId;
  }
  if (left.type === "branch" && right.type === "branch") {
    return left.branch === right.branch;
  }
  return true;
}

/**
 * Opens (or focuses) a Changes viewport tab for the given explorer context.
 * Dedupes by serialized context so the same context reuses one tab.
 */
export function openChangesTab(
  context?: FractalFileExplorerContext,
): string {
  const explorerContext =
    context ??
    aos.stores.files.state.explorerContext;

  const serialized = serializeExplorerContext(explorerContext);
  const tabs = aos.stores.viewport.state.tabs.items;

  const existing = tabs.find(
    (tab) =>
      tab.type === "changes" &&
      contextsMatch(getTabExplorerContext(tab), explorerContext),
  );

  if (existing) {
    aos.stores.viewport.actions.setActiveTab(existing.id);
    return existing.id;
  }

  const label = formatExplorerContextLabel(explorerContext);
  const title =
    explorerContext.type === "main" ? "Changes" : `Changes · ${label}`;

  const createdTabId = aos.stores.viewport.actions.createTab({
    type: "changes",
    title,
    closable: true,
    metadata: {
      fileExplorerContext: serialized,
    },
  });

  if (createdTabId) {
    aos.stores.viewport.actions.setActiveTab(createdTabId);
  }

  return createdTabId;
}
