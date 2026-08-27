import * as React from "react";
import { Link } from "@tanstack/react-router";
import { HugeiconsIcon } from "@hugeicons/react";
import { HomeIcon, SearchIcon } from "@hugeicons/core-free-icons";
import { Button } from "@/components/ui/button";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { stores } from "@/app/lib/stores";
import { useSidebarActiveRoute } from "../hooks/use-sidebar-active-route";
import { t } from "@/lib/i18n";

/**
 * Desktop chrome cluster for Home / Search.
 *
 * Mounted after back/forward in {@link WorkspaceNavControls}. Activity lives
 * on the right of {@link WorkspaceNavigation}. Mobile keeps Home / Search in
 * the sidebar Navigation group.
 */
export function WorkspaceNavOrientationControls() {
  const { isRouteActive } = useSidebarActiveRoute();

  const openSearch = React.useCallback(() => {
    stores.viewport.actions.toggleCommander(true);
  }, []);

  return (
    <div className="ml-0.5 flex shrink-0 items-center gap-0.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            asChild
            className={cn(
              "h-8 w-8 text-muted-foreground hover:text-foreground",
            )}
          >
            <Link to="/" aria-label={t("Home")}>
              <HugeiconsIcon icon={HomeIcon} className="size-3.5" />
            </Link>
          </Button>
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          align="start"
          className="flex items-center gap-2"
        >
          {t("Home")}
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground hover:text-foreground"
            onClick={openSearch}
            aria-label={t("Search")}
          >
            <HugeiconsIcon icon={SearchIcon} className="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent
          side="bottom"
          align="start"
          className="flex items-center gap-2"
        >
          {t("Search")}
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>K</Kbd>
          </KbdGroup>
        </TooltipContent>
      </Tooltip>
    </div>
  );
}
