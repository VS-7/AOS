"use client";

import * as React from "react";
import { Link } from "@tanstack/react-router";
import {
  Add01Icon,
  Clock02Icon,
  FullScreenIcon,
  GitBranchIcon,
  RefreshIcon,
  SidebarRightIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { cn } from "@/lib/utils";
import { useSidebar } from "@/components/ui/sidebar";
import { Button } from "@/components/ui/button";
import { aos } from "@/app/aos";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { useSidebarActiveRoute } from "../sidebar/hooks/use-sidebar-active-route";
import { WorkspaceTabStrip } from "./workspace-tab-strip";
import { WORKSPACE_NAV_CONTROLS_INSET_CLASS } from "../sidebar/components/workspace-nav-controls-shell";
import { WindowControls } from "@/components/ui/window-controls";
import { t } from "@/lib/i18n";

export function WorkspaceNavigation() {
  const clickTimeoutRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const { open: isMainSidebarOpen } = useSidebar();
  const isNative = typeof window !== "undefined" && !!window.aos;
  const { isRouteActive } = useSidebarActiveRoute();
  const unreadCount = aos.stores.activity.useState((s) => s.unreadCount);

  const fullscreen = aos.stores.viewport.useState((s) => s.fullscreen);
  const isPageSidebarVisible = aos.stores.viewport.useState(
    (s) => s.page.sidebar.visible,
  );
  const isPageSidebarEnabled = aos.stores.viewport.useState(
    (s) => s.page.sidebar.enabled,
  );
  const tabs = aos.stores.viewport.useState((s) => s.tabs.items);
  const activeTabId = aos.stores.viewport.useState((s) => s.tabs.current);

  const handleFullscreenAction = () => {
    if (clickTimeoutRef.current) {
      clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = null;

      // fullscreen() already decides the sidebar (hidden entering, shown
      // leaving). Toggling it again here undid that: a double-click entered
      // fullscreen with the sidebar still open, and left it with it closed.
      aos.stores.viewport.actions.fullscreen(
        fullscreen ? undefined : "page",
      );
    } else {
      clickTimeoutRef.current = setTimeout(() => {
        (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("viewport.fullscreen.page");
        clickTimeoutRef.current = null;
      }, 250);
    }
  };

  const handleTabClick = (tabId: string) => {
    aos.stores.viewport.actions.setActiveTab(tabId);
  };

  aos.triggers.use({
    trigger: "viewport.fullscreen.page",
  });

  aos.triggers.use({
    trigger: "viewport.toggle.details",
  });

  aos.triggers.use({
    trigger: "viewport.toggle.page_sidebar",
  });

  aos.triggers.use({
    trigger: "files.changes.open",
  });

  return (
    <div className="grid w-full min-w-0 grid-cols-[1fr_auto] items-center gap-1 pt-1 drag-region">
      <div
        className={cn(
          "flex min-w-0 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          "transition-[padding-left] duration-200 ease-linear",
          isMainSidebarOpen
            ? "pl-4"
            : isNative
              ? WORKSPACE_NAV_CONTROLS_INSET_CLASS.native
              : WORKSPACE_NAV_CONTROLS_INSET_CLASS.web,
        )}
      >
        <WorkspaceTabStrip
          tabs={tabs}
          activeTabId={activeTabId}
          onSelect={handleTabClick}
        />

        {isNative && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                aria-label={t("New Browser Tab")}
                onClick={() => {
                  aos.stores.viewport.actions.createTab();
                }}
                className="flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground no-drag"
              >
                <HugeiconsIcon icon={Add01Icon} className="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent
              side="bottom"
              align="start"
              className="flex items-center gap-2"
            >
              {t("New Browser Tab")}
              <KbdGroup>
                <Kbd>⌘</Kbd>
                <Kbd>T</Kbd>
              </KbdGroup>
            </TooltipContent>
          </Tooltip>
        )}
      </div>

      <div className="pointer-events-auto flex w-fit shrink-0 items-center gap-1 pr-4 no-drag">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              asChild
              className={cn(
                "relative h-8 w-8 text-muted-foreground hover:text-foreground",
                isRouteActive("/activities") && "text-foreground bg-accent/60",
              )}
            >
              <Link to="/activities" aria-label={t("Activity")}>
                <HugeiconsIcon icon={Clock02Icon} className="size-3.5" />
                {unreadCount > 0 ? (
                  <span className="absolute right-1 top-1 size-1.5 rounded-full bg-primary" />
                ) : null}
              </Link>
            </Button>
          </TooltipTrigger>
          <TooltipContent
            side="bottom"
            align="end"
            className="flex items-center gap-2"
          >
            {t("Activity")}
            {unreadCount > 0 ? (
              <span className="text-xs text-muted-foreground">
                {t("{{count}} unread", { count: unreadCount })}
              </span>
            ) : null}
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground disabled:opacity-20"
              aria-label={t("Open Changes")}
              onClick={() => (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("files.changes.open")}
            >
              <HugeiconsIcon icon={GitBranchIcon} className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent
            side="bottom"
            align="end"
            className="flex items-center gap-2"
          >
            {t("Open Changes")}
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>⇧</Kbd>
              <Kbd>C</Kbd>
            </KbdGroup>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground disabled:opacity-20"
              aria-label={t("Refresh")}
              onClick={() => (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tabs.reload")}
            >
              <HugeiconsIcon
                icon={RefreshIcon}
                className="size-3.5"
              />
            </Button>
          </TooltipTrigger>
          <TooltipContent
            side="bottom"
            align="end"
            className="flex items-center gap-2"
          >
            {t("Refresh")}
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>R</Kbd>
            </KbdGroup>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            {/* Styled like its neighbours at rest, and marked while on. It
                used to sit at 40% opacity — the look of a disabled control —
                and could never reach full opacity, because the panels its
                condition waited on are the ones fullscreen itself shows. */}
            <Button
              variant="ghost"
              size="icon"
              onClick={handleFullscreenAction}
              aria-label={t("Toggle Fullscreen")}
              aria-pressed={fullscreen === "page"}
              className={cn(
                "h-8 w-8 text-muted-foreground hover:text-foreground",
                fullscreen === "page" && "bg-accent/60 text-foreground",
              )}
            >
              <HugeiconsIcon icon={FullScreenIcon} className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent
            side="bottom"
            align="end"
            className="flex items-center gap-2"
          >
            {t("Toggle Fullscreen")}
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>⇧</Kbd>
              <Kbd>F</Kbd>
            </KbdGroup>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            {/* Only a page that has a sidebar can toggle one; on any other
                page (Home among them) the button used to look dimmed, toggle
                an invisible flag, and change nothing. */}
            <Button
              variant="ghost"
              size="icon"
              disabled={!isPageSidebarEnabled}
              onClick={() =>
                (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("viewport.toggle.page_sidebar")
              }
              aria-label={t("Toggle Page Sidebar")}
              aria-pressed={isPageSidebarEnabled ? isPageSidebarVisible : undefined}
              className={cn(
                "h-8 w-8 text-muted-foreground hover:text-foreground disabled:opacity-20",
                isPageSidebarEnabled && isPageSidebarVisible && "text-foreground",
              )}
            >
              <HugeiconsIcon icon={SidebarRightIcon} className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent
            side="bottom"
            align="end"
            className="flex items-center gap-2"
          >
            {t("Toggle Page Sidebar")}
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>⇧</Kbd>
              <Kbd>B</Kbd>
            </KbdGroup>
          </TooltipContent>
        </Tooltip>

        {/*
          * Minimise, maximise and close, on the platforms where the operating
          * system draws none — Windows and Linux, whose window is frameless.
          * It renders nothing on macOS, which keeps its own traffic lights,
          * and nothing in a browser tab. See components/ui/window-controls.
          */}
        <WindowControls className="-mr-4 ml-1" />
      </div>
    </div>
  );
}
