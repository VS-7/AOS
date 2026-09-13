import * as React from "react";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";
import { frameAddress, frameSandbox } from "@/lib/wails";
import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

/**
 * The browser tab's content, rendered in a plain iframe.
 *
 * This used to be an Electron `<webview>` element — `did-start-loading`,
 * `dom-ready`, `partition`, `executeJavaScript`, the works — ported from the
 * original's Electron shell along with a whole agent-automation bridge
 * (`window.aos.browser.{navigate,goBack,goForward,reload}`, plus
 * click/type/scroll/read-content) that expected it. Wails has no equivalent
 * embedded webview element, and no such bridge exists — window.d.ts already
 * documents `window.aos.browser` as permanently `undefined` here ("AOS has
 * no Wails bridge for either yet"). The `<webview>` tag itself was dead code:
 * an unrecognized custom element renders nothing, so every browser tab —
 * including every artifact opened from the sidebar — showed a blank pane
 * regardless of platform.
 *
 * An iframe needs no bridge at all for the page's own content, which is where
 * every artifact comes from (`/v/artifacts/{id}/*`, the app's own origin —
 * see internal/transport/artifactapi's own CSP, `frame-ancestors 'self'`,
 * which exists for exactly this). In the desktop window it is framed opaque,
 * at the address the window hands out for it (see `frameAddress`), since that
 * is the only one that serves its own files to an opaque page. It is a real,
 * working rendering path, not a stub: general web browsing to an arbitrary
 * external URL may still fail to display if the target site refuses to be
 * framed (`X-Frame-Options`/its own `frame-ancestors`) — an honest iframe
 * limitation, not something this component can detect or work around, and
 * strictly no worse than the total blank this replaces. Back/forward history
 * and the agent-automation bridge remain unimplemented — see tabs.trigger.ts
 * and browser/index.tsx's own comments on what still depends on the bridge
 * that does not exist.
 */
export function BrowserRenderer({
  tab,
  active,
  onStateChange,
}: {
  tab: ViewportTabState;
  active: boolean;
  onStateChange: (tabId: string, patch: Partial<ViewportTabState>) => void;
}) {
  const handleLoad = React.useCallback(
    (event: React.SyntheticEvent<HTMLIFrameElement>) => {
      let title = tab.title;
      try {
        // Unreadable for anything framed opaque (every artifact) or
        // cross-origin (an external page): left as the tab's existing title
        // rather than surfaced as an error — the page did load, this is just
        // cosmetic.
        const doc = event.currentTarget.contentDocument;
        if (doc?.title) title = doc.title;
      } catch {
        // Cross-origin: nothing more to read.
      }
      onStateChange(tab.id, { status: "idle", title, error: null });
    },
    [onStateChange, tab.id, tab.title],
  );

  const handleError = React.useCallback(() => {
    onStateChange(tab.id, { status: "idle", error: "Navigation failed." });
  }, [onStateChange, tab.id]);

  React.useEffect(() => {
    if (!tab.url) return;
    onStateChange(tab.id, { status: "loading", error: null });
    // Only tab.url and tab.reloadNonce should restart the loading indicator —
    // a title/status patch from handleLoad must not re-trigger this effect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab.url, tab.reloadNonce]);

  // Where the frame actually points, for the URL it was resolved from. Kept
  // together so a tab whose URL changed never shows the previous address.
  const [frame, setFrame] = React.useState<{ from: string; src: string } | null>(null);
  React.useEffect(() => {
    const url = tab.url;
    if (!url) return;
    let current = true;
    frameAddress(url).then(
      (src) => {
        if (current) setFrame({ from: url, src });
      },
      (error: unknown) => {
        if (!current) return;
        console.error(`[browser] no frame address for ${url}`, error);
        onStateChange(tab.id, { status: "idle", error: t("This artifact could not be opened.") });
      },
    );
    return () => {
      current = false;
    };
    // Resolved again only for a new URL; onStateChange is the parent's and
    // tab.id does not change for the life of the tab.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab.url]);
  const src = tab.url && frame?.from === tab.url ? frame.src : null;

  return (
    <div
      className={cn(
        "absolute inset-0 h-full w-full flex flex-col",
        !active && "pointer-events-none opacity-0",
      )}
    >
      {src ? (
        <iframe
          key={`${tab.id}:${tab.reloadNonce ?? 0}`}
          src={src}
          title={tab.title}
          className="flex-1 w-full border-0 bg-background"
          onLoad={handleLoad}
          onError={handleError}
          // No popups, no top-level navigation out from inside the frame. A
          // general external site that needs more than this to function will
          // not fully work in-frame — see this file's own top comment. In the
          // desktop window the window's own content (an artifact) is also
          // denied its origin, which would reach the Wails bridge — see
          // frameSandbox, which also says why a browser tab is not yet.
          sandbox={frameSandbox(src)}
        />
      ) : null}
    </div>
  );
}
