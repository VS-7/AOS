// Fractal's original ran under an Electron-style shell that exposed
// `window.fractal.instructions.openPath(...)` for opening a local file with
// the OS's default app. AOS is a Wails app with no equivalent bridge, so
// `window.fractal` is always `undefined` here — real Wails file-opening
// would be a separate, later integration, not a stub to fake now.
//
// This only declares the shape so `attachments/components/item.component.tsx`
// (ported, already guards every access with `?.`) typechecks; it does not
// claim the capability exists.
export {};

declare global {
  interface Window {
    fractal?: {
      instructions?: {
        openPath?: (path: string) => Promise<void>;
      };
    };
  }
}
