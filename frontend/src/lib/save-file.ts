/**
 * Saving a file the interface produced.
 *
 * Seven places build a Blob and hand it to `<a download>` — export a table to
 * CSV, save an image from a conversation, download mcp.json. That is what an
 * Electron renderer supports and what the port carried over. A WebView does
 * not: downloading needs a delegate on every platform (WKDownloadDelegate,
 * WebView2's DownloadStarting, WebKitGTK's decide-destination) and Wails
 * implements none of them, so the anchor click was accepted and nothing was
 * ever written. No error, no dialog, no file.
 *
 * Inside the application the bytes go over the bridge to
 * `wailsvc.SystemService.SaveFile`, which opens the operating system's own
 * save panel and writes where the person said. In a browser the anchor is
 * still right and is what runs.
 */
import { Call } from "@wailsio/runtime";
import { isDesktopWindow } from "./wails";

const WAILSVC_PKG = "github.com/OWNER/aos/internal/transport/wailsvc";

/** Matches `maxSaveBytes` in internal/transport/wailsvc/system.go. */
const MAX_BYTES = 64 * 1024 * 1024;

/** What a save attempt did, so the caller can say so. */
export type SaveResult =
  | { status: "saved"; path: string }
  | { status: "cancelled" }
  | { status: "failed"; reason: string };

/**
 * Base64 for the bridge, which carries JSON.
 *
 * Chunked rather than `String.fromCharCode(...bytes)`: spreading a multi-
 * megabyte array into a call overflows the engine's argument limit, and an
 * exported table or a saved image is routinely that size.
 */
function toBase64(bytes: Uint8Array): string {
  const CHUNK = 0x8000;
  let binary = "";
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
}

/** The browser's own download, unchanged — this is correct in a tab. */
function saveViaAnchor(blob: Blob, filename: string): SaveResult {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.style.display = "none";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
  return { status: "saved", path: filename };
}

/**
 * Saves `blob` under `filename`, wherever this is running.
 *
 * Never throws: a caller is a click handler, and a rejected promise there is a
 * console entry nobody reads. The result says what happened instead.
 */
export async function saveBlob(blob: Blob, filename: string): Promise<SaveResult> {
  if (!isDesktopWindow) return saveViaAnchor(blob, filename);

  if (blob.size > MAX_BYTES) {
    return {
      status: "failed",
      reason: "This file is too large to save through the window.",
    };
  }

  try {
    const bytes = new Uint8Array(await blob.arrayBuffer());
    const path = (await Call.ByName(
      `${WAILSVC_PKG}.SystemService.SaveFile`,
      filename,
      toBase64(bytes),
    )) as string;

    // "" is a cancelled panel, which is an answer and not a failure — the
    // same contract PickFiles has for an empty selection.
    return path ? { status: "saved", path } : { status: "cancelled" };
  } catch (error) {
    return {
      status: "failed",
      reason: error instanceof Error ? error.message : "The file could not be saved.",
    };
  }
}

/** Saves text under `filename`, with the media type the reader expects. */
export function saveText(
  text: string,
  filename: string,
  mediaType = "text/plain;charset=utf-8",
): Promise<SaveResult> {
  return saveBlob(new Blob([text], { type: mediaType }), filename);
}
