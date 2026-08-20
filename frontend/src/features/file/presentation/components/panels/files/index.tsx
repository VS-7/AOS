import React from "react";
import { aos } from "@/app/aos";
import { FilesContent } from "./components/content";
import { cn } from "@/lib/utils";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

/**
 * Viewport panel that keep-alive mounts every open `file` tab and only
 * visually hides inactive ones, preserving editor state across switches.
 */
export function FilePanel() {
  const activeTabId = aos.stores.viewport.useState(
    (state) => state.tabs.current,
  );
  const fileTabs = aos.stores.viewport.useState((state) =>
    state.tabs.items.filter((tab) => tab.type === "file"),
  );
  const activeTab = aos.stores.viewport.useState((state) =>
    state.tabs.items.find((tab) => tab.id === activeTabId),
  );
  const isFileSurfaceActive = activeTab?.type === "file";

  if (fileTabs.length === 0) return null;

  return (
    <div
      className={cn(
        "relative flex-1 h-full min-h-0 min-w-0 overflow-hidden",
        !isFileSurfaceActive && "hidden",
      )}
    >
      {fileTabs.map((tab) => (
        <FileTabSurface
          key={tab.id}
          tab={tab}
          active={tab.id === activeTabId}
        />
      ))}
    </div>
  );
}

function FileTabSurface({
  tab,
  active,
}: {
  tab: ViewportTabState;
  active: boolean;
}) {
  return (
    <div
      className={cn(
        "absolute inset-0 min-h-0 min-w-0 overflow-hidden",
        !active && "hidden",
      )}
    >
      <FilesContent
        activeFilePath={
          typeof tab.metadata?.filePath === "string"
            ? tab.metadata.filePath
            : undefined
        }
        activeFileTabId={tab.id}
        activeFileTabMetadata={tab.metadata}
      />
    </div>
  );
}
