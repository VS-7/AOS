/**
 * Where an HTTP call to the daemon has to be addressed.
 *
 * In a browser the daemon serves this page, so a relative path is right and
 * this resolves to the empty string, leaving `/api/...` exactly as written.
 *
 * In the desktop window it is wrong, and silently so. The page comes from the
 * application binary's own embedded assets, which the webview serves from
 * `wails://localhost` — so `fetch("/api/file/tree")` reaches the asset host,
 * which has no such route, and answers with the interface's own index.html.
 * That is a 200 with HTML in it, so nothing throws: the caller parses it as
 * JSON, fails, and reports "the daemon answered 200 with something that is not
 * JSON" — if it reports anything at all.
 *
 * Everything that rides the command registry avoids this by going over the
 * Wails bridge instead. What does not is exactly what has no bridge: the file
 * explorer and editor (`lib/file.ts`), and the identity endpoints that
 * `AuthService` does not bind (`lib/auth.ts`). Those are the file tree, file
 * read and write, diffs, and the account roster — every one of which simply
 * did not work in the desktop window.
 *
 * The address is the one the window states in its own URL when it opens
 * (`cmd/aos-desktop`'s WebviewWindowOptions.URL), read once at module load
 * because the router rewrites that URL on the first navigation. It is the same
 * value `lib/realtime.ts` reads, for the same reason.
 */
const declared =
  typeof window === "undefined"
    ? null
    : new URLSearchParams(window.location.search).get("daemon");

/** The origin to prefix an API path with. Empty in a browser. */
export const daemonOrigin: string = declared ? declared.replace(/\/+$/, "") : "";

/**
 * An absolute URL for a daemon path.
 *
 * Takes a root-relative path (`/api/file/tree?...`) and returns it unchanged in
 * a browser, or prefixed with the daemon's origin inside the desktop window.
 */
export function daemonURL(path: string): string {
  if (!daemonOrigin) return path;
  return daemonOrigin + (path.startsWith("/") ? path : "/" + path);
}
