import * as React from "react";
import { cn } from "@/lib/utils";
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
 * An iframe needs no bridge at all for same-origin content, which is what
 * every artifact this daemon serves is (`/v/artifacts/{id}/*`, same origin
 * as the app itself — see internal/transport/artifactapi's own CSP,
 * `frame-ancestors 'self'`, which exists for exactly this). It is a real,
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
        // Only readable for same-origin content (every artifact); a
        // cross-origin external page throws here, left as the tab's
        // existing title rather than surfaced as an error — the page did
        // load, this is just cosmetic.
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

  return (
    <div
      className={cn(
        "absolute inset-0 h-full w-full flex flex-col",
        !active && "pointer-events-none opacity-0",
      )}
    >
      {tab.url ? (
        <iframe
          key={`${tab.id}:${tab.reloadNonce ?? 0}`}
          src={tab.url}
          title={tab.title}
          className="flex-1 w-full border-0 bg-background"
          onLoad={handleLoad}
          onError={handleError}
          // Same posture as the artifact CSP this most often points at:
          // same-origin, no popups, no top-level navigation out from inside
          // the frame. A general external site that needs more than this
          // to function will not fully work in-frame — see this file's own
          // top comment.
          sandbox="allow-scripts allow-same-origin allow-forms"
        />
      ) : null}
    </div>
  );
}
