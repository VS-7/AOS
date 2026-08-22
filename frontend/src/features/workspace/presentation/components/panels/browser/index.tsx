import * as React from "react";
import { aos } from "@/app/aos";
import { Page, PageSecondaryHeader } from "@/components/ui/page";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { BROWSER_HOME_URL } from "@/features/workspace/presentation/stores/browser.store";
import { BrowserToolbar } from "./components/toolbar.component";
import { BrowserViewport } from "./components/viewport.component";
import { cn } from "@/lib/utils";

/**
 * The browser panel: one address bar plus every open "browser"-type tab,
 * rendered by BrowserViewport/BrowserRenderer (a plain iframe — see that
 * file's own comment on why, and on what still doesn't work without a
 * native embed this app doesn't have: back/forward history, and the
 * agent-automation bridge (`window.aos.browser`'s read-content/click/type/
 * scroll) the original's Electron shell exposed. Both were previously
 * "wired" here against that bridge, which window.d.ts already documents as
 * permanently undefined — dead code, removed rather than left pretending to
 * work. `tabs.back`/`tabs.forward` (tabs.trigger.ts) are the same
 * documented gap; `tabs.reload`/`tabs.navigate` do work now, through the
 * viewport store instead of the bridge.
 */
export const BrowserPanel = () => {
  const tabs = aos.stores.viewport.useState((state) => state.tabs.items.filter(t => t.type === 'browser'));
  const activeId = aos.stores.viewport.useState((state) => state.tabs.current);
  const activeTab = aos.stores.viewport.useState((state) => state.tabs.items.find((tab) => tab.id === activeId) || null);
  const addressBarValue = aos.stores.browser.useState((state) => state.ui.addressBarValue);
  const shouldFocusAddressBar = aos.stores.browser.useState((state) => state.ui.isAddressBarFocused);

  const addressBarRef = React.useRef<HTMLInputElement | null>(null);

  React.useEffect(() => {
    if (!activeTab || activeTab.type !== 'browser') return;
    aos.stores.browser.actions.setAddressBarValue(activeTab.url || "");
  }, [activeTab?.id, activeTab?.url]);

  React.useEffect(() => {
    if (!shouldFocusAddressBar) return;
    addressBarRef.current?.focus();
    addressBarRef.current?.select();
    aos.stores.browser.actions.setAddressBarFocused(false);
  }, [shouldFocusAddressBar]);

  aos.triggers.use({
    trigger: "tabs.newTab",
    onPressKey: () => aos.stores.viewport.actions.createTab()
  });

  aos.triggers.use({
    trigger: "tabs.closeTab",
    enabled: tabs.length > 0,
    onPressKey: () => aos.stores.viewport.actions.closeTab(activeId)
  });

  aos.triggers.use({
    trigger: "tabs.focusAddress",
  });

  aos.triggers.use({
    trigger: "tabs.reload",
    enabled: !!activeTab,
  });

  const handleNavigate = React.useCallback(async () => {
    const nextValue = addressBarRef.current?.value || addressBarValue;
    if (!activeId || !nextValue.trim()) return;

    void (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.navigate", { url: nextValue });
  }, [activeId, addressBarValue]);

  const handleUpdateTab = React.useCallback((tabId: string, patch: Partial<ViewportTabState>) => {
    aos.stores.viewport.actions.updateTab(tabId, patch);
  }, []);

  return (
    <Page className={cn([
      "flex-1 h-full min-w-0 overflow-hidden relative",
      activeTab?.type !== "browser" && "hidden"
    ])}>
      <PageSecondaryHeader className="h-12 px-6 pointer-events-none border-border/20">
        <div className="pointer-events-auto w-full">
          <BrowserToolbar
            activeTab={activeTab}
            addressBarValue={addressBarValue}
            addressBarRef={addressBarRef}
            onNavigate={handleNavigate}
            onReload={() => void (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.reload")}
            onAddressBarChange={(value) => aos.stores.browser.actions.setAddressBarValue(value)}
            onAddressBarFocus={(focused) => aos.stores.browser.actions.setAddressBarFocused(focused)}
            onGoHome={() => {
              void (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.navigate", { url: BROWSER_HOME_URL });
            }}
          />
        </div>
      </PageSecondaryHeader>

      <BrowserViewport
        tabs={tabs}
        activeId={activeId}
        activeTab={activeTab}
        onUpdateTab={handleUpdateTab}
      />
    </Page>
  );
};
