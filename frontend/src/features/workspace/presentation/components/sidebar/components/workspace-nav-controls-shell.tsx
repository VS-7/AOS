import * as React from "react";
import { cn } from "@/lib/utils";
import { useSidebar } from "@/components/ui/sidebar";
import { WorkspaceNavControls } from "./workspace-nav-controls";

/** Tab strip left padding when the main sidebar is collapsed (clears fixed controls). */
export const WORKSPACE_NAV_CONTROLS_INSET_CLASS = {
  native: "ml-[16rem]",
  web: "ml-[11rem]",
} as const;

export function WorkspaceNavControlsShell() {
  const isNative =
    typeof window !== "undefined" && !!(window as Window & { fractal?: unknown }).fractal;
  const { open: isMainSidebarOpen } = useSidebar();

  return (
    <div
      data-slot="workspace-nav-controls-shell"
      className={cn(
        "fixed top-0 left-[var(--workspace-rail-width)] z-30 flex h-14 items-center px-2 py-3",
        "[-webkit-app-region:no-drag]",
        isNative && "pl-20",
        isMainSidebarOpen
          ? "w-[var(--sidebar-width)] bg-sidebar"
          : "bg-transparent",
      )}
    >
      <WorkspaceNavControls />
    </div>
  );
}
