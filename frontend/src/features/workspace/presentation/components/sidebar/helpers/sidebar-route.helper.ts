export type SidebarRouteActiveOptions = {
  exact?: boolean;
};

/**
 * Returns whether `pathname` matches a sidebar route target.
 * Use `exact: true` for home (`/`) and other leaf-only links.
 */
export function isRouteActive(
  pathname: string,
  to: string,
  options?: SidebarRouteActiveOptions,
): boolean {
  if (options?.exact) {
    return pathname === to;
  }

  if (to === "/") {
    return pathname === "/";
  }

  return pathname === to || pathname.startsWith(`${to}/`);
}

/**
 * Extracts the first dynamic segment after a route prefix (e.g. `/collections/foo` → `foo`).
 */
export function getRouteSegmentId(
  pathname: string,
  routePrefix: string,
): string | undefined {
  const prefix = routePrefix.endsWith("/")
    ? routePrefix
    : `${routePrefix}/`;

  if (!pathname.startsWith(prefix)) {
    return undefined;
  }

  const segment = pathname.slice(prefix.length).split("/")[0];

  if (!segment) {
    return undefined;
  }

  return decodeURIComponent(segment);
}

export function getCurrentChatId(pathname: string): string | undefined {
  return getRouteSegmentId(pathname, "/chats");
}
