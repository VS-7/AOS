/**
 * Minimise, maximise and close, drawn by the interface.
 *
 * The window is frameless on Windows and Linux, which means the operating
 * system draws no title bar and no controls at all: without these three there
 * is no way to close, minimise or resize the window with the mouse. That was
 * the state the application shipped in — `Frameless: true` on every platform,
 * and no chrome drawn anywhere in the interface to replace what it removed.
 *
 * macOS is not frameless (see `cmd/aos-desktop`'s framelessHere) and keeps its
 * own traffic lights, so this renders nothing there. Drawing a second set
 * beside the real ones is worse than drawing none.
 */
import { useCallback, useEffect, useState, type JSX } from "react";
import { Events } from "@wailsio/runtime";
import { cn } from "@/lib/utils";
import {
  closeWindow,
  isWindowMaximised,
  minimiseWindow,
  needsWindowControls,
  toggleMaximiseWindow,
  whenPlatformKnown,
} from "@/lib/wails";

function Glyph({ d, filled }: { d: string; filled?: boolean }): JSX.Element {
  return (
    <svg
      width="10"
      height="10"
      viewBox="0 0 10 10"
      aria-hidden="true"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="1.1"
      strokeLinecap="square"
    >
      <path d={d} />
    </svg>
  );
}

interface ControlProps {
  label: string;
  region: "minimise" | "maximise" | "close";
  onClick: () => void;
  danger?: boolean;
  children: JSX.Element;
}

function Control({ label, region, onClick, danger, children }: ControlProps): JSX.Element {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      /*
       * Two class names, two different mechanisms — see styles/app.css.
       *
       * `no-drag` is what makes this clickable at all on macOS and Linux: it
       * sits inside the title bar's drag region, and a pointer press on a
       * draggable element starts moving the window instead of reaching the
       * button.
       *
       * `window-control-*` is what makes it behave like a real caption button
       * on Windows, where hit testing happens natively before the webview
       * sees the press — including the Snap Layouts flyout the maximise
       * button is expected to open on hover.
       */
      className={cn(
        "no-drag flex h-8 w-11 shrink-0 items-center justify-center",
        `window-control-${region}`,
        "text-muted-foreground transition-colors",
        danger
          ? "hover:bg-destructive hover:text-white"
          : "hover:bg-muted hover:text-foreground",
      )}
    >
      {children}
    </button>
  );
}

export function WindowControls({ className }: { className?: string }): JSX.Element | null {
  // Not decided once at mount: the platform arrives with the Wails runtime
  // configuration, which the host injects after this bundle has run (see
  // lib/wails.ts). Until it does, `needsWindowControls` answers false, which
  // is the safe way round — macOS must never draw a second set of controls
  // beside its own, and Windows and Linux can wait a frame for theirs.
  const [shown, setShown] = useState(needsWindowControls);
  const [maximised, setMaximised] = useState(false);

  useEffect(() => whenPlatformKnown(() => setShown(needsWindowControls())), []);

  useEffect(() => {
    if (!shown) return;
    let cancelled = false;

    const sync = () => {
      void isWindowMaximised().then((value) => {
        if (!cancelled) setMaximised(value);
      });
    };
    sync();

    // The window can also be maximised by dragging it to the top of the
    // screen or double-clicking the title bar, neither of which goes through
    // the button below. Wails reports all three transitions; `resize` stays as
    // the fallback for a host that does not, and costs one bridge call when it
    // fires.
    const off = [
      Events.On(Events.Types.Common.WindowMaximise, sync),
      Events.On(Events.Types.Common.WindowUnMaximise, sync),
      Events.On(Events.Types.Common.WindowRestore, sync),
    ];
    window.addEventListener("resize", sync);

    return () => {
      cancelled = true;
      for (const cancel of off) cancel();
      window.removeEventListener("resize", sync);
    };
  }, [shown]);

  const onToggle = useCallback(() => {
    void toggleMaximiseWindow().then(() =>
      isWindowMaximised().then(setMaximised),
    );
  }, []);

  if (!shown) return null;

  return (
    <div
      data-slot="window-controls"
      className={cn("flex items-center", className)}
    >
      <Control label="Minimise" region="minimise" onClick={() => void minimiseWindow()}>
        <Glyph d="M1 5h8" />
      </Control>
      <Control
        label={maximised ? "Restore" : "Maximise"}
        region="maximise"
        onClick={onToggle}
      >
        {maximised ? (
          <Glyph d="M2.5 3.5h4v4h-4zM3.5 2.5h4v4" />
        ) : (
          <Glyph d="M1.5 1.5h7v7h-7z" />
        )}
      </Control>
      <Control label="Close" region="close" danger onClick={() => void closeWindow()}>
        <Glyph d="M1.5 1.5l7 7M8.5 1.5l-7 7" />
      </Control>
    </div>
  );
}
