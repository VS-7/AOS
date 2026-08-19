import type { FractalFileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import {
  buildFileTabMetadata,
  synthesizeFileFromPath,
} from "@/features/file/presentation/helpers/files-explorer.helper";
import { aos } from "@/app/aos";

function getTabFilePath(tab: {
  metadata?: Record<string, string | number | boolean>;
}): string | undefined {
  const value = tab.metadata?.filePath;
  return typeof value === "string" ? value : undefined;
}

/**
 * Opens (or focuses) a workspace-relative file path in a viewport file tab.
 */
export function openWorkspaceFileTab(
  relativePath: string,
  options?: {
    explorerContext?: FractalFileExplorerContext;
    readOnly?: boolean;
    title?: string;
  },
): string | undefined {
  const normalized = relativePath.replace(/^\.\//, "").replace(/\/+$/, "");
  if (!normalized) return undefined;

  const explorerContext = options?.explorerContext ?? { type: "main" };
  const file = synthesizeFileFromPath(normalized, []);
  const metadata = buildFileTabMetadata(file, explorerContext, {
    fileReadOnly: options?.readOnly ?? false,
  });

  const tabs = aos.stores.viewport.state.tabs.items;
  const existingTab = tabs.find(
    (tab: { type: string; id: string; metadata?: Record<string, string | number | boolean> }) =>
      tab.type === "file" && getTabFilePath(tab) === file.path,
  );

  if (existingTab) {
    aos.stores.viewport.actions.setActiveTab(existingTab.id);
    return existingTab.id;
  }

  const createdTabId = aos.stores.viewport.actions.createTab({
    type: "file",
    title: options?.title ?? file.name,
    closable: true,
    metadata,
  });

  if (createdTabId) {
    aos.stores.viewport.actions.setActiveTab(createdTabId);
  }

  return createdTabId;
}
