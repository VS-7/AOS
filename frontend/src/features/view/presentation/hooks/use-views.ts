import * as React from "react";
import { useNavigate, useRouterState } from "@tanstack/react-router";

import { ViewStore } from "@/features/view/presentation/stores/view.store";
import { skillSearch } from "@/lib/skill-scope";

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
  const currentSkill = useRouterState({
    select: (state) => {
      const skill = (state.location.search as { skill?: unknown } | undefined)?.skill;
      return typeof skill === "string" && skill ? skill : undefined;
    },
  });

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

  // A skill's view goes with its skill in the address: ids are unique only
  // within a scope, and the id alone answers NOT_FOUND for a skill's view.
  function openView(viewId: string, skill?: string) {
    void navigate({ to: "/views/$id", params: { id: viewId }, search: skillSearch(skill) });
  }

  /** Whether `view` is the one on screen: same id, and the same scope. */
  function isCurrent(view: { id: string; skill?: string }) {
    if (currentViewId !== view.id) return false;
    // An address without a skill opens whichever entry it resolves to; with
    // one, only that skill's view is the current one.
    return currentSkill === undefined || currentSkill === (view.skill || undefined);
  }

  return {
    current: currentViewId,
    currentSkill,
    isCurrent,
    open: openView,
    views,
  };
}