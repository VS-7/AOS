/**
 * The Wails v3 runtime, as this interface uses it.
 *
 * The screens in this application were ported from one that ran under
 * Electron, and the port carried Electron's assumptions about what a page can
 * do. Most of them are not Electron APIs — they are ordinary browser APIs that
 * Electron's renderer happens to implement and a WebView does not — which is
 * why none of them failed loudly. They returned a falsy answer and the feature
 * simply did nothing.
 *
 * Every one of the following was verified against the installed runtime
 * (`@wailsio/runtime@3.0.0-beta.8`) and against the Wails source it talks to
 * (`wails/v3@v3.0.0-beta.8`), not inferred from the documentation site — which
 * describes a different revision in at least one load-bearing detail.
 *
 *   - `window.confirm` / `alert` / `prompt`. WKWebView routes these to its
 *     `WKUIDelegate`. Wails' delegate (`webview_window_darwin.h`) conforms to
 *     the protocol but implements exactly one of its methods —
 *     `runOpenPanelWithParameters`, for file inputs. There is no
 *     `runJavaScriptConfirmPanelWithMessage:`, so on macOS `confirm()` returns
 *     `false` without ever drawing anything, and every destructive action
 *     guarded by one was unreachable. `Dialogs.Question` is the replacement.
 *
 *   - `window.open` and `target="_blank"`. Both need the same delegate's
 *     `createWebViewWithConfiguration:forNavigationAction:windowFeatures:`,
 *     which is likewise absent, so both are silently inert. `Browser.OpenURL`
 *     hands the URL to the operating system, which is what these meant.
 *
 *   - The page's own query string. The desktop window is opened at
 *     `wails://localhost/?daemon=…` (see `cmd/aos-desktop`'s
 *     WebviewWindowOptions.URL) and that parameter is the only thing telling
 *     the interface where the daemon is. A `location.replace("/")` drops it,
 *     which cost the window its API origin and its event channel until the
 *     application was restarted. `desktopURL` keeps it.
 *
 * What is deliberately *not* wrapped: `navigator.clipboard`. All three
 * platforms serve this page from a `localhost` host (`assetserver_darwin.go`,
 * `_linux.go`, `_windows.go`), which is a potentially-trustworthy origin, so
 * the API is present. It can still reject when a call has lost the user
 * gesture behind an `await`, and `installClipboardFallback` covers exactly
 * that, without touching the 28 call sites that are already correct.
 */
import { Browser, Clipboard, Dialogs, System, Window } from "@wailsio/runtime";

/** Go's GOOS, which is what the Wails runtime reports. */
export type Platform = "darwin" | "windows" | "linux" | "";

const params =
  typeof window === "undefined"
    ? new URLSearchParams()
    : new URLSearchParams(window.location.search);

/**
 * Whether this page is the desktop window.
 *
 * Read once at module load, because the router rewrites the URL on the first
 * navigation. `System.IsDesktop()` cannot answer this: it reads
 * `window._wails.environment`, which the host injects from its
 * `WebViewDidFinishNavigation` hook (`webview_window_darwin.go`'s
 * `newWindowImpl`) — that is, *after* this bundle has run. It answers false
 * during exactly the window this has to be right for.
 */
export const isDesktopWindow: boolean =
  typeof window !== "undefined" && params.has("daemon");

/**
 * The seed value for the platform, stated by the window that opened this page.
 *
 * The same reasoning as above: `System.IsMac()` is authoritative but not yet
 * answerable at first paint, and the alternative the interface used before —
 * matching `/Macintosh/` against the user agent — is a guess about a fact the
 * process on the other side of the bridge already knows for certain.
 */
const declaredPlatform = (params.get("platform") ?? "") as Platform;

/**
 * The operating system this window is running on, or "" in a browser tab.
 *
 * Prefers the Wails runtime once the host has injected its environment, and
 * falls back to what the window stated in its own URL before that happens.
 * The two always agree — they come from the same `runtime.GOOS` in the same
 * process — so this is a timing fallback, not a disagreement to resolve.
 */
export function platform(): Platform {
  if (System.IsMac()) return "darwin";
  if (System.IsWindows()) return "windows";
  if (System.IsLinux()) return "linux";
  return declaredPlatform;
}

export function isMac(): boolean {
  return platform() === "darwin";
}

/**
 * Runs `listener` once the Wails environment is available, and again never.
 *
 * The runtime dispatches `wails:runtime-config-ready` on `window` from a
 * microtask right after the host injects its configuration (`runtime.go`'s
 * `runtimeConfigReady`). Anything that has to change once the platform is
 * known — rather than merely read it once — subscribes here.
 *
 * @returns a function that cancels the subscription.
 */
export function whenPlatformKnown(listener: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  if (platform() !== "") {
    listener();
    return () => {};
  }
  window.addEventListener("wails:runtime-config-ready", listener, { once: true });
  return () => window.removeEventListener("wails:runtime-config-ready", listener);
}

/* -------------------------------------------------------------------------
 * Navigation
 * ---------------------------------------------------------------------- */

/**
 * A URL for a hard navigation that keeps this window's identity.
 *
 * In a browser this is `path`, unchanged. In the desktop window it carries the
 * parameters the window was opened with — the daemon's address above all,
 * which nothing else on the page can recover.
 *
 * @param path - a root-relative path, with or without its own query string.
 */
export function desktopURL(path: string): string {
  if (!isDesktopWindow) return path;

  const url = new URL(path, window.location.href);
  for (const key of ["daemon", "platform"]) {
    const value = params.get(key);
    if (value !== null && !url.searchParams.has(key)) {
      url.searchParams.set(key, value);
    }
  }
  return url.pathname + url.search + url.hash;
}

/**
 * Reloads the interface at `path`, keeping the desktop window's parameters.
 *
 * Use this rather than `window.location.replace` anywhere the intent is "start
 * the application over at this route" — switching workspace, finishing
 * onboarding. A bare `replace("/")` re-enters the desktop window as if it were
 * a browser tab: no daemon origin for `/api/*`, no event channel, and
 * `window.aos` never installed, so the layout reverts to its browser shape.
 */
export function reloadAt(path: string): void {
  window.location.replace(desktopURL(path));
}

/* -------------------------------------------------------------------------
 * The operating system's own facilities
 * ---------------------------------------------------------------------- */

/**
 * Whether `url` names somewhere other than this page's own origin.
 *
 * Not "does it start with http": the page's own origin is `wails://localhost`
 * in a built application but `http://localhost:9245` under `wails3 dev`, so a
 * scheme test sends every internal route in the development window out to the
 * system browser. Comparing origins is right in all three: production, the dev
 * server, and a browser tab.
 */
function isExternal(url: string): boolean {
  if (/^(mailto|tel):/i.test(url)) return true;
  if (!/^https?:\/\//i.test(url)) return false;
  try {
    return new URL(url).origin !== window.location.origin;
  } catch {
    return false;
  }
}

/**
 * Opens `url` in the system browser.
 *
 * In a browser tab this is a new window, as before. In the desktop window it
 * is `Browser.OpenURL`, because `window.open` there returns null and opens
 * nothing.
 */
export async function openExternal(url: string): Promise<void> {
  if (!isDesktopWindow) {
    window.open(url, "_blank", "noopener,noreferrer");
    return;
  }
  try {
    await Browser.OpenURL(url);
  } catch (error) {
    // The bridge is gone or the URL was refused. Nothing else can open it,
    // and a thrown rejection from a click handler is an unhandled rejection
    // in the console rather than anything the person sees.
    console.error(`[wails] could not open ${url} externally`, error);
  }
}

/**
 * Sends every external link click to the operating system.
 *
 * There are roughly twenty `target="_blank"` anchors in this interface, plus
 * every link inside a rendered markdown document — which is user content, so
 * the set is open-ended and cannot be fixed by editing call sites. All of them
 * are inert in the desktop window for the reason at the top of this file.
 *
 * The listener is on the capture phase so it sees the click before a component
 * can stop it propagating, and it defers to anything that has already called
 * `preventDefault` — a link a component is handling itself is not a link this
 * should hijack.
 *
 * @returns a function that removes the listener.
 */
export function installExternalLinkHandler(): () => void {
  if (!isDesktopWindow || typeof document === "undefined") return () => {};

  const onClick = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button !== 0) return;

    const target = event.target;
    if (!(target instanceof Element)) return;

    const anchor = target.closest("a[href]");
    if (!(anchor instanceof HTMLAnchorElement)) return;

    // `href` on an anchor is already resolved against the document, so this
    // is the absolute URL — a relative route reads back as `wails://…`, which
    // is not external and is left to the router.
    const href = anchor.href;
    if (!isExternal(href)) return;

    event.preventDefault();
    void openExternal(href);
  };

  document.addEventListener("click", onClick, { capture: true });
  return () => document.removeEventListener("click", onClick, { capture: true });
}

/**
 * Makes a rejected `navigator.clipboard.writeText` fall through to Wails.
 *
 * WebKit requires a live user gesture for a clipboard write, and a handler
 * that awaits anything first — fetching the token it is about to copy, say —
 * has spent it by the time it asks. `Clipboard.SetText` goes over the bridge
 * to the host process, which has no such requirement.
 *
 * This wraps rather than replaces: a write that succeeds on its own never
 * reaches Wails, so the browser keeps its ordinary behaviour and only the
 * failure path changes.
 */
export function installClipboardFallback(): void {
  if (!isDesktopWindow || typeof navigator === "undefined") return;
  if (!navigator.clipboard?.writeText) return;

  const original = navigator.clipboard.writeText.bind(navigator.clipboard);
  try {
    Object.defineProperty(navigator.clipboard, "writeText", {
      configurable: true,
      value: async (text: string): Promise<void> => {
        try {
          await original(text);
        } catch {
          await Clipboard.SetText(text);
        }
      },
    });
  } catch {
    // A host that will not let the property be redefined. Nothing is lost —
    // the original stays, which is what would have run anyway. This must not
    // throw: it is installed from `main.tsx` before React mounts, so a
    // rejection here would be a blank window instead of a missing fallback.
  }
}

/**
 * Asks the person to confirm something destructive, natively.
 *
 * Prefer the in-app dialog (`useAlert().confirm`) wherever there is a React
 * tree to render it into: it looks like the rest of the application and
 * behaves identically in a browser. This is for the places that have no such
 * tree, and for a browser tab, where `window.confirm` is still the honest
 * answer.
 *
 * A rejection resolves to `false`. Refusing to act is the safe reading of "the
 * dialog did not happen" for a caller that is about to delete something.
 */
export async function confirmNatively(options: {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
}): Promise<boolean> {
  const confirmLabel = options.confirmLabel ?? "OK";
  const cancelLabel = options.cancelLabel ?? "Cancel";

  if (!isDesktopWindow) {
    return window.confirm(
      options.message ? `${options.title}\n\n${options.message}` : options.title,
    );
  }

  try {
    const chosen = await Dialogs.Question({
      Title: options.title,
      Message: options.message ?? "",
      Buttons: [
        { Label: confirmLabel },
        { Label: cancelLabel, IsCancel: true, IsDefault: true },
      ],
    });
    return chosen === confirmLabel;
  } catch (error) {
    console.error("[wails] the confirmation dialog could not be shown", error);
    return false;
  }
}

/* -------------------------------------------------------------------------
 * The window itself
 * ---------------------------------------------------------------------- */

/**
 * Which of the three window controls this interface has to draw.
 *
 * macOS draws its own: the window is not frameless there, and the traffic
 * lights are the real ones, inset over full-size content (see
 * `cmd/aos-desktop`'s `framelessHere`). Windows and Linux have no equivalent —
 * their frame is all or nothing — so the window is frameless and these three
 * are the only way to close it with the mouse.
 */
export function needsWindowControls(): boolean {
  return isDesktopWindow && platform() !== "darwin" && platform() !== "";
}

export async function minimiseWindow(): Promise<void> {
  await Window.Minimise();
}

export async function toggleMaximiseWindow(): Promise<void> {
  await Window.ToggleMaximise();
}

export async function closeWindow(): Promise<void> {
  await Window.Close();
}

export async function isWindowMaximised(): Promise<boolean> {
  try {
    return await Window.IsMaximised();
  } catch {
    return false;
  }
}
