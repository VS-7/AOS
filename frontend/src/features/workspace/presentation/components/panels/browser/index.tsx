import * as React from "react";
import { aos } from "@/app/aos";
import { Page, PageSecondaryHeader } from "@/components/ui/page";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";
import { BROWSER_HOME_URL } from "@/features/workspace/presentation/stores/browser.store";
import { BrowserToolbar } from "./components/toolbar.component";
import { BrowserViewport } from "./components/viewport.component";
import type { BrowserWebViewElement } from "./components/renderer.component";
import { cn } from "@/lib/utils";

type BrowserCommandPayload = { tabId?: string; url?: string };
type BrowserRequestPayload = { requestId: string; tabId?: string };
type BrowserClickPayload = BrowserRequestPayload & { selector: string };
type BrowserTypePayload = BrowserRequestPayload & { selector: string; text: string };
type BrowserScrollPayload = BrowserRequestPayload & { direction: "up" | "down"; amount?: number };

export const BrowserPanel = () => {
  const tabs = aos.stores.viewport.useState((state) => state.tabs.items.filter(t => t.type === 'browser'));
  const activeId = aos.stores.viewport.useState((state) => state.tabs.current);
  const activeTab = aos.stores.viewport.useState((state) => state.tabs.items.find((tab) => tab.id === activeId) || null);
  const addressBarValue = aos.stores.browser.useState((state) => state.ui.addressBarValue);
  const shouldFocusAddressBar = aos.stores.browser.useState((state) => state.ui.isAddressBarFocused);

  const addressBarRef = React.useRef<HTMLInputElement | null>(null);
  const webviewsRef = React.useRef<Record<string, BrowserWebViewElement | null>>({});
  const isNative = typeof window !== "undefined" && !!window.aos?.browser;

  const navigateWebview = React.useCallback((tabId: string, url: string) => {
    const webview = webviewsRef.current[tabId];
    if (!webview) return;

    const isReady = webview.getAttribute("data-dom-ready") === "true";

    if (!isReady) {
      webview.setAttribute("data-pending-url", url);
    } else {
      webview.removeAttribute("data-pending-url");
    }

    if (webview.getAttribute("src") !== url) {
      webview.setAttribute("src", url);
    }
  }, []);

  const resolveTargetWebview = React.useCallback(
    (tabId?: string) => {
      const resolvedTabId = tabId || aos.stores.viewport.state.tabs.current;

      if (!resolvedTabId) {
        return { error: "No browser tab is available for this action." } as const;
      }

      const tab = aos.stores.viewport.state.tabs.items.find((candidate) => candidate.id === resolvedTabId);

      if (tab?.type !== "browser") {
        return { error: `Tab '${resolvedTabId}' is not an open browser tab.` } as const;
      }

      const webview = webviewsRef.current[resolvedTabId];
      if (!webview) {
        return { error: `Browser tab '${resolvedTabId}' is not ready or was already closed.` } as const;
      }

      return { resolvedTabId, webview } as const;
    },
    []
  );

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

  React.useEffect(() => {
    if (!window.aos?.browser) return;
    const browser = window.aos.browser;

    const unsubscribers = [
      browser.on("navigate", (payload?: BrowserCommandPayload) => {
        const tabId = payload?.tabId || aos.stores.viewport.state.tabs.current;
        if (!tabId || !payload?.url) return;
        navigateWebview(tabId, payload.url);
      }),

      browser.on("goBack", (payload?: BrowserCommandPayload) => {
        const tabId = payload?.tabId || aos.stores.viewport.state.tabs.current;
        if (!tabId) return;
        const webview = webviewsRef.current[tabId];
        if (webview?.canGoBack?.()) {
          webview.goBack();
        }
      }),

      browser.on("goForward", (payload?: BrowserCommandPayload) => {
        const tabId = payload?.tabId || aos.stores.viewport.state.tabs.current;
        if (!tabId) return;
        const webview = webviewsRef.current[tabId];
        if (webview?.canGoForward?.()) {
          webview.goForward();
        }
      }),

      browser.on("reload", (payload?: BrowserCommandPayload) => {
        const tabId = payload?.tabId || aos.stores.viewport.state.tabs.current;
        if (!tabId) return;
        const webview = webviewsRef.current[tabId];
        webview?.reload?.();
      }),

      browser.on("focus-address-bar", () => {
        aos.stores.browser.actions.focusAddressBar();
      }),

      // [Agent Tools]: Listen for automation requests from the BrowserService
      browser.on("browser:request:read-content", async (payload: BrowserRequestPayload) => {
        const target = resolveTargetWebview(payload.tabId);
        if ("error" in target) {
          browser.emit(`browser:response:${payload.requestId}`, { error: target.error });
          return;
        }

        try {
          const data = await target.webview.executeJavaScript("document.body.innerText");
          browser.emit(`browser:response:${payload.requestId}`, { data });
        } catch (error) {
          browser.emit(`browser:response:${payload.requestId}`, { error: String(error) });
        }
      }),

      browser.on("browser:request:click", async (payload: BrowserClickPayload) => {
        const target = resolveTargetWebview(payload.tabId);
        if ("error" in target) {
          browser.emit(`browser:response:${payload.requestId}`, { error: target.error });
          return;
        }

        try {
          const selector = JSON.stringify(payload.selector);
          await target.webview.executeJavaScript(`
            const element = document.querySelector(${selector});
            if (!element) {
              throw new Error("No element matched the provided selector.");
            }
            element.click();
          `);
          browser.emit(`browser:response:${payload.requestId}`, { data: true });
        } catch (error) {
          browser.emit(`browser:response:${payload.requestId}`, { error: String(error) });
        }
      }),

      browser.on("browser:request:type", async (payload: BrowserTypePayload) => {
        const target = resolveTargetWebview(payload.tabId);
        if ("error" in target) {
          browser.emit(`browser:response:${payload.requestId}`, { error: target.error });
          return;
        }

        try {
          const selector = JSON.stringify(payload.selector);
          const text = JSON.stringify(payload.text);
          await target.webview.executeJavaScript(`
            const element = document.querySelector(${selector});
            if (!element) {
              throw new Error("No element matched the provided selector.");
            }
            element.value = ${text};
            element.dispatchEvent(new Event("input", { bubbles: true }));
            element.dispatchEvent(new Event("change", { bubbles: true }));
          `);
          browser.emit(`browser:response:${payload.requestId}`, { data: true });
        } catch (error) {
          browser.emit(`browser:response:${payload.requestId}`, { error: String(error) });
        }
      }),

      browser.on("browser:request:scroll", async (payload: BrowserScrollPayload) => {
        const target = resolveTargetWebview(payload.tabId);
        if ("error" in target) {
          browser.emit(`browser:response:${payload.requestId}`, { error: target.error });
          return;
        }

        try {
          const amount = payload.amount || (payload.direction === "down" ? 500 : -500);
          await target.webview.executeJavaScript(`window.scrollBy(0, ${amount})`);
          browser.emit(`browser:response:${payload.requestId}`, { data: true });
        } catch (error) {
          browser.emit(`browser:response:${payload.requestId}`, { error: String(error) });
        }
      }),
    ];

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  }, [navigateWebview, resolveTargetWebview]);

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

  const handleSetWebviewRef = React.useCallback((tabId: string, node: BrowserWebViewElement | null) => {
    webviewsRef.current[tabId] = node;
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
        isNative={isNative}
        tabs={tabs}
        activeId={activeId}
        activeTab={activeTab}
        onUpdateTab={handleUpdateTab}
        onSetWebviewRef={handleSetWebviewRef}
      />
    </Page>
  );
};
