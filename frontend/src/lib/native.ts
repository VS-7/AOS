/**
 * The desktop bridge, as the ported interface expects to find it.
 *
 * The screens came from an application whose shell exposed `window.fractal.*`,
 * and they ask `window.aos` the same questions: am I the desktop, reveal this
 * path, sync the window material with this theme. Eight components read
 * `!!window.aos` synchronously on their first render to decide the sidebar
 * inset that clears the macOS traffic lights, whether the window material is
 * translucent, and whether the new-tab control exists — so this is installed
 * from `main.tsx` before React mounts, and never later.
 *
 * Everything platform-shaped lives in `lib/wails.ts`, which is where the
 * reasoning about what a WebView can and cannot do is written down. This file
 * is only the shape the ported code reaches for, laid over it.
 */
import { Call, Events } from "@wailsio/runtime";
import { system } from "./client";
import {
  closeWindow,
  installClipboardFallback,
  installExternalLinkHandler,
  isDesktopWindow,
  isWindowMaximised,
  minimiseWindow,
  reloadHere,
  toggleMaximiseWindow,
} from "./wails";

/**
 * The window event the native menu's View › Reload sends, instead of
 * reloading the webview itself — see cmd/aos-desktop's applicationMenu.
 */
export const RELOAD_EVENT = "aos:reload";

/**
 * What the page answers RELOAD_EVENT with before it reloads itself. The window
 * reloads itself when this does not arrive — see cmd/aos-desktop's pageReload.
 */
export const RELOAD_ACK_EVENT = "aos:reload-ack";

/**
 * The page's side of the menu's Reload: say it was heard, then reload.
 *
 * Said first, while the bridge is still there to carry it; a reload that went
 * first would take the acknowledgement down with the page, and the window
 * would reload a second time at the URL it was opened with. A bridge that
 * cannot carry it changes nothing here: the page reloads either way.
 */
async function acknowledgeAndReload(): Promise<void> {
  try {
    await Events.Emit(RELOAD_ACK_EVENT);
  } catch {
    // The window will reload itself as well, which is still a reload.
  }
  reloadHere();
}

const WAILSVC_PKG = "github.com/OWNER/aos/internal/transport/wailsvc";

export { isDesktopWindow };

/**
 * The theme store's mode, in the words SystemService.SetAppearance accepts.
 *
 * The store says light, dark or system; the Go side says light, dark or auto
 * (internal/transport/wailsvc/system.go). "system" went across as it was and
 * was refused on every page load — AOS_SYSTEM_UNKNOWN_APPEARANCE, twice,
 * swallowed by the fire-and-forget below — so the native window never
 * followed the theme. Anything that is not a definite light or dark is auto.
 */
export function nativeAppearance(mode: string): "light" | "dark" | "auto" {
  return mode === "light" || mode === "dark" ? mode : "auto";
}

/**
 * Installs `window.aos`, and the two document-wide corrections the port needs.
 *
 * Idempotent, and a no-op outside the desktop window: in a browser tab
 * `window.aos` must stay undefined, because that is what every `isNative` read
 * means by it.
 */
export function installNativeBridge(): void {
  if (!isDesktopWindow || typeof window === "undefined") return;
  if (window.aos) return;

  // Both of these fix whole classes of call site rather than one each — see
  // their own comments in lib/wails.ts for why editing the call sites is not
  // the available option.
  installExternalLinkHandler();
  installClipboardFallback();

  // Reload keeps the window's route and parameters when the page does it; the
  // window only reloads itself for a page that never got this far.
  Events.On(RELOAD_EVENT, () => void acknowledgeAndReload());

  window.aos = {
    instructions: {
      async openPath(path: string): Promise<void> {
        await Call.ByName(`${WAILSVC_PKG}.SystemService.OpenPath`, path);
      },
    },
    system: {
      async showItemInFolder(path: string): Promise<boolean> {
        try {
          await Call.ByName(`${WAILSVC_PKG}.SystemService.RevealInFolder`, path);
          return true;
        } catch {
          // The path is outside the workspace, or no workspace is known yet
          // — both are refusals, not failures the person needs a dialog for.
          // The caller renders the menu item as unavailable on false.
          return false;
        }
      },
    },
    theme: {
      setAppearance({ mode, windows }: { mode: string; windows: string }) {
        // Fire-and-forget: the CSS has already been applied by the caller, and
        // the native material catching up a frame later is invisible. A
        // rejection here is a window that has gone away.
        void system.setAppearance(nativeAppearance(mode), windows).catch(() => {});
      },
    },
    window: {
      minimise: minimiseWindow,
      toggleMaximise: toggleMaximiseWindow,
      close: closeWindow,
      isMaximised: isWindowMaximised,
    },
    // `browser` is deliberately absent. The embedded-webview control the
    // ported browser panel drives (navigate/goBack/goForward/reload over a
    // second webview) has no Wails equivalent: Wails v3 has one webview per
    // window and no `<webview>` tag, which is what the Electron original
    // used. The ported code guards every access with `?.` precisely so the
    // panel degrades instead of throwing. Declaring a stub that resolves to
    // nothing would turn a visibly-missing feature into a silently broken one.
  };
}
