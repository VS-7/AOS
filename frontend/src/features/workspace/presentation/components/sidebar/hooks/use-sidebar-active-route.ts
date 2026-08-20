import * as React from "react";
import { useRouterState } from "@tanstack/react-router";
import {
  getCurrentChatId,
  getRouteSegmentId,
  isRouteActive,
  type SidebarRouteActiveOptions,
} from "../helpers/sidebar-route.helper";

export function useSidebarActiveRoute() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });

  return React.useMemo(
    () => ({
      pathname,
      isRouteActive: (to: string, options?: SidebarRouteActiveOptions) =>
        isRouteActive(pathname, to, options),
      getRouteSegmentId: (routePrefix: string) =>
        getRouteSegmentId(pathname, routePrefix),
      getCurrentChatId: () => getCurrentChatId(pathname),
      getCurrentCollectionName: () => getRouteSegmentId(pathname, "/collections"),
      getCurrentProjectId: () => getRouteSegmentId(pathname, "/projects"),
      getCurrentViewId: () => getRouteSegmentId(pathname, "/views"),
    }),
    [pathname],
  );
}
