import { platform } from "@/lib/wails";

/**
 * Whether the platform's command key is ⌘ (macOS) rather than Control.
 *
 * It matters because on macOS Control is text editing — ^N next line, ^K
 * delete to the end, ^B back a character — so a shortcut that takes Control
 * there takes those keys away from every field. The window states its
 * platform; a browser tab states none, and its own `navigator` is the only
 * witness left.
 */
export function commandKeyIsMeta(): boolean {
  const declared = platform();
  if (declared) return declared === "darwin";
  return typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
}

/** Whether the platform's command key is held, and not the other one. */
export function commandKeyHeld(
  event: Pick<KeyboardEvent, "metaKey" | "ctrlKey">,
  mac: boolean = commandKeyIsMeta(),
): boolean {
  return mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
}
