import * as React from "react";
import { useNavigate, useRouterState } from "@tanstack/react-router";

import { ViewStore } from "@/features/view/presentation/stores/view.store";

function getCurrentViewId(pathname: string): string | undefined {
  if (!pathname.startsWith("/views/")) {
    return undefined;
  }

  return decodeURIComponent(pathname.replace("/views/", "").split("/")[0] || "");
}

export function useViews() {
  const navigate = useNavigate();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const currentViewId = getCurrentViewId(pathname);

  const rawViews = ViewStore.useState((state) => state.items);

  const views = React.useMemo(() => {
    return [...rawViews].sort((a, b) => {
      const orderA =
        typeof a.metadata?.order === "number"
          ? a.metadata.order
          : Number.MAX_SAFE_INTEGER;
      const orderB =
        typeof b.metadata?.order === "number"
          ? b.metadata.order
          : Number.MAX_SAFE_INTEGER;

      if (orderA !== orderB) {
        return orderA - orderB;
      }

      return a.title.localeCompare(b.title);
    });
  }, [rawViews]);

  function openView(viewId: string) {
    void navigate({ to: "/views/$id", params: { id: viewId } });
  }

  return {
    current: currentViewId,
    open: openView,
    views,
  };
}