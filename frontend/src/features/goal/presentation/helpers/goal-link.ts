import { t } from "@/lib/i18n";
import { isDesktopWindow } from "@/lib/wails";

/**
 * Hosts a desktop window is served from. Wails v3 serves the page as
 * `wails://localhost` on macOS and Linux and as `http://wails.localhost` on
 * Windows (`internal/assetserver/assetserver_*.go`): the Windows one is
 * http, and still not an address anything outside the window can open.
 */
const DESKTOP_ASSET_HOSTS = new Set(["wails.localhost"]);

/**
 * What "Copy link" can honestly copy for a record at `path`.
 *
 * The page's origin is only an address when it is a browser's — this bundle
 * served over http(s) by the daemon or through a tunnel. In the desktop
 * window it is the webview's asset host, and a link built on it opens
 * nothing anywhere else, so the action copies the in-app path instead and
 * says so. The window is recognised by the parameters it was opened with
 * (`isDesktopWindow`), and by its asset host in case those were lost.
 */
export function shareableLink(
  path: string,
  origin: string | undefined = globalThis.location?.origin,
  desktop: boolean = isDesktopWindow,
) {
  if (!desktop && origin && /^https?:\/\//.test(origin) && !DESKTOP_ASSET_HOSTS.has(new URL(origin).hostname)) {
    return { value: new URL(path, origin).toString(), label: t("Copy link"), copied: t("Link copied") };
  }
  return { value: path, label: t("Copy path"), copied: t("Path copied") };
}
