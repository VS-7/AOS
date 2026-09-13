import { t } from "@/lib/i18n";

/**
 * What "Copy link" can honestly copy for a record at `path`.
 *
 * The page's origin is only an address when it is http(s) — a browser, or the
 * window served over HTTP. Inside the desktop window it is the webview's own
 * asset scheme, and a link built on it opens nothing anywhere else, so the
 * action copies the in-app path instead and says so.
 */
export function shareableLink(path: string, origin: string | undefined = globalThis.location?.origin) {
  if (origin && /^https?:\/\//.test(origin)) {
    return { value: new URL(path, origin).toString(), label: t("Copy link"), copied: t("Link copied") };
  }
  return { value: path, label: t("Copy path"), copied: t("Path copied") };
}
