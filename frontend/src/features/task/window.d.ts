// The original ran under an Electron-style shell that exposed
// `window.fractal.instructions.openPath(...)` for opening a local file with
// the OS's default app. AOS is a Wails app with no equivalent bridge, so
// `window.aos` is always `undefined` here — real Wails file-opening
// would be a separate, later integration, not a stub to fake now.
//
// This only declares the shape so `attachments/components/item.component.tsx`
// (ported, already guards every access with `?.`) typechecks; it does not
// claim the capability exists.
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
      theme?: {
        setAppearance?: (params: { mode: string; windows: string; surface?: unknown }) => void;
      };
    };
  }
}
