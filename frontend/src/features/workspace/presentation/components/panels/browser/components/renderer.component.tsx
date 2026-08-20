import * as React from "react";
import { cn } from "@/lib/utils";
import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

export interface BrowserWebViewElement extends HTMLElement {
  src: string;
  canGoBack: () => boolean;
  canGoForward: () => boolean;
  goBack: () => void;
  goForward: () => void;
  reload: () => void;
  isLoading: () => boolean;
  getURL: () => string;
  getTitle: () => string;
  executeJavaScript: (code: string) => Promise<any>;
}

declare global {
  namespace JSX {
    interface IntrinsicElements {
      webview: React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement> & {
        allowpopups?: boolean;
        partition?: string;
        src?: string;
        useragent?: string;
        webpreferences?: string;
      };
    }
  }
}

const DESKTOP_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36";

export function BrowserRenderer({
  tab,
  active,
  onStateChange,
  onRefChange,
}: {
  tab: ViewportTabState;
  active: boolean;
  onStateChange: (tabId: string, patch: Partial<ViewportTabState>) => void;
  onRefChange: (tabId: string, node: BrowserWebViewElement | null) => void;
}) {
  const ref = React.useRef<BrowserWebViewElement | null>(null);
  const isReadyRef = React.useRef(false);
  const pendingUrlRef = React.useRef<string | null>(null);
  const latestTabRef = React.useRef(tab);

  React.useEffect(() => {
    latestTabRef.current = tab;
  }, [tab]);

  React.useEffect(() => {
    onRefChange(tab.id, ref.current);
    return () => onRefChange(tab.id, null);
  }, [onRefChange, tab.id]);

  React.useEffect(() => {
    const webview = ref.current;
    if (!webview) return;

    // Apply desktop user agent immediately
    webview.setAttribute("useragent", DESKTOP_USER_AGENT);

    isReadyRef.current = false;
    pendingUrlRef.current = tab.url || null;
    webview.setAttribute("data-dom-ready", "false");
    webview.setAttribute("data-pending-url", tab.url || "");

    const syncState = (patch?: Partial<ViewportTabState>, options?: { preserveUrl?: boolean; preserveTitle?: boolean }) => {
      const currentTab = latestTabRef.current;
      const isReady = isReadyRef.current || webview.getAttribute("data-dom-ready") === "true";

      if (!isReady) {
        onStateChange(tab.id, {
          url: currentTab.url,
          title: currentTab.title,
          status: patch?.status ?? currentTab.status,
          canGoBack: false,
          canGoForward: false,
          error: patch?.error ?? null,
          ...patch,
        });
        return;
      }

      let nextUrl = currentTab.url;
      let nextTitle = currentTab.title;
      let nextIsLoading = currentTab.status === 'loading'
      let nextCanGoBack = currentTab.canGoBack;
      let nextCanGoForward = currentTab.canGoForward;

      try {
        nextUrl = options?.preserveUrl ? currentTab.url : webview.getURL?.() || currentTab.url;
        nextTitle = options?.preserveTitle ? currentTab.title : webview.getTitle?.() || currentTab.title;
        nextIsLoading = webview.isLoading?.() ?? currentTab.status === 'loading'
        nextCanGoBack = webview.canGoBack?.() ?? currentTab.canGoBack;
        nextCanGoForward = webview.canGoForward?.() ?? currentTab.canGoForward;
      } catch {
        nextUrl = currentTab.url;
        nextTitle = currentTab.title;
        nextIsLoading = patch ? patch.status === 'loading' : currentTab.status === 'loading'
        nextCanGoBack = currentTab.canGoBack;
        nextCanGoForward = currentTab.canGoForward;
      }

      onStateChange(tab.id, {
        url: nextUrl,
        title: nextTitle,
        status: patch?.status ?? currentTab.status,
        canGoBack: nextCanGoBack,
        canGoForward: nextCanGoForward,
        error: null,
        ...patch,
      });
    };

    const handleStart = () => {
      syncState({ status: 'loading', error: null }, { preserveUrl: true, preserveTitle: true });
    };
    const handleStop = () => {
      syncState({ status: 'idle' });
    };
    const handleNavigate = () => {
      syncState();
    };
    const handleTitle = () => syncState();
    const handleFavicon = (event: any) => {
      const favicons = event.favicons;
      if (favicons && favicons.length > 0) {
        syncState({ favicon: favicons[0] });
      }
    };
    const handleDomReady = () => {
      isReadyRef.current = true;
      webview.setAttribute("data-dom-ready", "true");

      const pendingUrl = pendingUrlRef.current;
      const attributePendingUrl = webview.getAttribute("data-pending-url");
      const nextUrl = attributePendingUrl || pendingUrl;

      if (!nextUrl) {
        syncState();
        return;
      }

      const currentUrl = webview.getURL?.() || "";
      pendingUrlRef.current = null;
      webview.removeAttribute("data-pending-url");

      if (currentUrl && currentUrl === nextUrl) {
        syncState();
        return;
      }

      if (webview.getAttribute("src") !== nextUrl) {
        webview.setAttribute("src", nextUrl);
      }
    };
    const handleFail = (event: Event) => {
      const detail = event as Event & { errorCode?: number; errorDescription?: string };

      if (detail.errorCode === -3 || detail.errorDescription === "ERR_ABORTED") {
        syncState({
          status: 'idle',
          error: null,
        });
        return;
      }

      syncState({
        status: 'idle',
        error: detail.errorDescription || "Navigation failed.",
      });
    };

    webview.addEventListener("did-start-loading", handleStart as EventListener);
    webview.addEventListener("did-stop-loading", handleStop as EventListener);
    webview.addEventListener("did-navigate", handleNavigate as EventListener);
    webview.addEventListener("did-navigate-in-page", handleNavigate as EventListener);
    webview.addEventListener("page-title-updated", handleTitle as EventListener);
    webview.addEventListener("page-favicon-updated", handleFavicon as EventListener);
    webview.addEventListener("dom-ready", handleDomReady as EventListener);
    webview.addEventListener("did-fail-load", handleFail as EventListener);

    return () => {
      isReadyRef.current = false;
      webview.removeAttribute("data-dom-ready");
      webview.removeAttribute("data-pending-url");
      webview.removeEventListener("did-start-loading", handleStart as EventListener);
      webview.removeEventListener("did-stop-loading", handleStop as EventListener);
      webview.removeEventListener("did-navigate", handleNavigate as EventListener);
      webview.removeEventListener("did-navigate-in-page", handleNavigate as EventListener);
      webview.removeEventListener("page-title-updated", handleTitle as EventListener);
      webview.removeEventListener("page-favicon-updated", handleFavicon as EventListener);
      webview.removeEventListener("dom-ready", handleDomReady as EventListener);
      webview.removeEventListener("did-fail-load", handleFail as EventListener);
    };
  }, [onStateChange, tab.id]);

  React.useEffect(() => {
    const webview = ref.current;
    if (!webview || !tab.url) return;

    if (!isReadyRef.current) {
      pendingUrlRef.current = tab.url;
      webview.setAttribute("data-pending-url", tab.url);
      if (webview.getAttribute("src") !== tab.url) {
        webview.setAttribute("src", tab.url);
      }
      return;
    }

    const currentUrl = webview.getURL?.() || "";
    if (currentUrl !== tab.url) {
      pendingUrlRef.current = null;
      webview.removeAttribute("data-pending-url");
      if (webview.getAttribute("src") !== tab.url) {
        webview.setAttribute("src", tab.url);
      }
    }
  }, [tab.url]);

  return (
    <div
      className={cn(
        "absolute inset-0 h-full w-full flex flex-col",
        !active && "pointer-events-none opacity-0"
      )}
    >
      <webview
        ref={(node) => {
          ref.current = node as BrowserWebViewElement | null;
        }}
        src={tab.url}
        partition="persist:aos-browser"
        useragent={DESKTOP_USER_AGENT}
        webpreferences="contextIsolation=yes,nodeIntegration=no,allowRunningInsecureContent=yes"
        allowpopups
        className="flex-1 w-full"
      />
    </div>
  );
}
