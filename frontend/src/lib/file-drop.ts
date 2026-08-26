/**
 * Files dragged onto the desktop window.
 *
 * The interface was ported with HTML5 drop handlers — `dragover`/`drop` on the
 * composer's form and on the document, reading `event.dataTransfer.files` — and
 * they are correct in a browser. Inside the application they never run: every
 * platform routes a file drag to the native window before the WebView sees it,
 * and on macOS the window registers itself as the dragging destination
 * outright (`webview_window_darwin.m`'s `setDelegate`). The drop was accepted
 * by the window and thrown away.
 *
 * What replaces them is the Wails path: the window is opened with
 * `EnableFileDrop`, the drop target is marked with `data-file-drop-target` so
 * the runtime can find it, and the host reports the drop as a window event
 * carrying paths (`cmd/aos-desktop`'s FilesDroppedEventName). Paths, not
 * bytes — so this reads each one back through the daemon to rebuild the `File`
 * objects the composer works in.
 *
 * A file outside the workspace comes back marked as such rather than read: the
 * desktop's file access is confined to the workspace it is looking at (see
 * `wailsvc.SystemService.inside`), and that policy is not one worth breaking
 * for a drag. It is said out loud instead of failing quietly.
 */
import { useEffect } from "react";
import { Events } from "@wailsio/runtime";
import { read } from "./file";
import { isDesktopWindow } from "./wails";

/** The event name `cmd/aos-desktop` relays a drop under. */
const FILES_DROPPED = "aos:files-dropped";

/**
 * The attribute Wails' runtime looks for to decide a drop landed somewhere
 * that wants it (`getDropTargetElement` in the runtime's window.js). Without
 * it on an ancestor of the drop point, the runtime discards the drop before
 * the host ever hears about it.
 */
export const FILE_DROP_TARGET_ATTRIBUTE = "data-file-drop-target";

/** One path, as `wailsvc.DroppedFile` sends it. */
interface DroppedFile {
  name: string;
  path: string;
  inside: boolean;
}

/** What the caller is told about a drop it cannot have. */
export interface RefusedDrop {
  names: string[];
}

function decodeBase64(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

/**
 * Reads one workspace file back into a `File`.
 *
 * The daemon answers text or base64, never both (`internal/domain/file.Content`),
 * which is why this checks `base64` first rather than guessing from the media
 * type: a `.svg` is text and a `.png` is not, and the daemon has already made
 * that decision once.
 */
async function fileFrom(dropped: DroppedFile): Promise<File | null> {
  try {
    const content = await read(dropped.path);
    const type = content.mediaType || "application/octet-stream";
    const body =
      content.base64 !== undefined && content.base64 !== ""
        ? [decodeBase64(content.base64) as BlobPart]
        : [(content.text ?? "") as BlobPart];
    return new File(body, dropped.name, { type });
  } catch {
    // Deleted between the drag and the read, or unreadable. One missing file
    // does not spoil the rest of the drop.
    return null;
  }
}

/**
 * Delivers files dragged onto the window to `onFiles`.
 *
 * A no-op in a browser tab, where the ordinary `drop` handlers already work
 * and this would double every attachment.
 *
 * @param onFiles - called with the files that could be read, if any.
 * @param onRefused - called when the drop included files outside the workspace.
 * @param enabled - lets a caller stand down while it is not the drop target.
 */
export function useNativeFileDrop(
  onFiles: (files: File[]) => void,
  onRefused?: (refused: RefusedDrop) => void,
  enabled = true,
): void {
  useEffect(() => {
    if (!enabled || !isDesktopWindow) return;
    let cancelled = false;

    const off = Events.On(FILES_DROPPED, (event: { data?: unknown }) => {
      // Wails delivers a single emitted value as a one-element array in some
      // versions and bare in others; accept both rather than depend on which.
      const payload = Array.isArray(event?.data) ? event.data[0] : event?.data;
      const dropped = (Array.isArray(payload) ? payload : []) as DroppedFile[];
      if (dropped.length === 0) return;

      const outside = dropped.filter((file) => !file.inside);
      if (outside.length > 0) {
        onRefused?.({ names: outside.map((file) => file.name) });
      }

      const inside = dropped.filter((file) => file.inside);
      if (inside.length === 0) return;

      void Promise.all(inside.map(fileFrom)).then((files) => {
        if (cancelled) return;
        const readable = files.filter((file): file is File => file !== null);
        if (readable.length > 0) onFiles(readable);
      });
    });

    return () => {
      cancelled = true;
      off();
    };
  }, [enabled, onFiles, onRefused]);
}
