// The bridge the interface reaches the desktop through.
//
// The original ran under an Electron-style shell that exposed
// `window.fractal.*`. AOS is a Wails application, and `lib/native.ts` installs
// the same shape over the calls `internal/transport/wailsvc.SystemService`
// exposes plus the window controls from the Wails runtime — so inside the
// desktop window `window.aos` is defined, and in a browser tab it is not.
//
// That distinction is load-bearing: eight places in the interface read
// `!!window.aos` as "am I the desktop", and decide the sidebar inset that
// clears the macOS traffic lights, whether the window material is translucent,
// and whether the new-tab control exists. Leaving it permanently undefined —
// which it was, since nothing assigned it — made the desktop lay itself out as
// a browser tab.
//
// Every member stays optional. `browser` genuinely has no implementation (see
// lib/native.ts on why a stub would be worse), and the ported code guards
// every access with `?.` already.
export {};

declare global {
  interface Window {
    aos?: {
      instructions?: {
        openPath?: (path: string) => Promise<void>;
      };
      // Task 9 additions — same "declares the shape, doesn't claim the
      // capability" spirit as `instructions` above. `browser` (embedded
      // webview control) and `system.showItemInFolder` (OS file reveal)
      // are read by the freshly-copied workspace browser panel and file
      // tree context menu; AOS has no Wails bridge for either yet, so
      // `window.aos` stays `undefined` in practice and every access
      // is already `?.`-guarded by the ported code.
      browser?: {
        reload: (params: { tabId: string }) => void;
        goBack: (params: { tabId: string }) => void;
        goForward: (params: { tabId: string }) => void;
        navigate: (params: { tabId: string; url: string }) => void;
        on: (event: string, handler: (payload: any) => void) => () => void;
        emit: (event: string, payload?: any) => void;
      };
      system?: {
        showItemInFolder?: (path: string) => Promise<boolean>;
      };
      /**
       * The frame the interface draws for itself where the operating system
       * draws none — Windows and Linux. See components/ui/window-controls.
       */
      window?: {
        minimise: () => Promise<void>;
        toggleMaximise: () => Promise<void>;
        close: () => Promise<void>;
        isMaximised: () => Promise<boolean>;
      };
      theme?: {
        setAppearance?: (params: { mode: string; windows: string; surface?: unknown }) => void;
      };
    };
  }
}
