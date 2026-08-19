import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import { ChangesContent } from "./components/changes-content";
import { getChangesTabContext } from "@/features/file/presentation/helpers/changes.helper";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

/**
 * Viewport panel that keep-alive mounts every open `changes` tab and only
 * visually hides inactive ones, preserving diff UI state across switches.
 */
export function ChangesPanel() {
  const activeTabId = aos.stores.viewport.useState(
    (state) => state.tabs.current,
  );
  const changesTabs = aos.stores.viewport.useState((state) =>
    state.tabs.items.filter((tab) => tab.type === "changes"),
  );
  const activeTab = aos.stores.viewport.useState((state) =>
    state.tabs.items.find((tab) => tab.id === activeTabId),
  );
  const isChangesSurfaceActive = activeTab?.type === "changes";

  if (changesTabs.length === 0) return null;

  return (
    <div
      className={cn(
        "relative flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
        !isChangesSurfaceActive && "hidden",
      )}
    >
      {changesTabs.map((tab) => (
        <ChangesTabSurface
          key={tab.id}
          tab={tab}
          active={tab.id === activeTabId}
        />
      ))}
    </div>
  );
}

function ChangesTabSurface({
  tab,
  active,
}: {
  tab: ViewportTabState;
  active: boolean;
}) {
  const explorerContext = getChangesTabContext(tab.metadata);

  return (
    <div
      className={cn(
        "absolute inset-0 flex min-h-0 min-w-0 flex-col overflow-hidden",
        !active && "hidden",
      )}
    >
      <ChangesContent tabId={tab.id} explorerContext={explorerContext} />
    </div>
  );
}
