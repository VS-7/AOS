import * as React from "react";
import { KbdGroup, Kbd } from "@/components/ui/kbd";
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";
import { HugeiconsIcon } from "@hugeicons/react";
import { SearchIcon, HomeIcon } from "@hugeicons/core-free-icons";
import { stores } from "@/app/lib/stores";
import { useIsMobile } from "@/hooks/use-mobile";
import { useSidebarActiveRoute } from "../../../../../hooks/use-sidebar-active-route";
import { t } from "@/lib/i18n";

/**
 * Mobile-only Home / Search rows.
 *
 * Desktop moves these into the top chrome via {@link WorkspaceNavOrientationControls}.
 * Activity lives on the right of {@link WorkspaceNavigation} for all form factors.
 */
export function WorkspaceSidebarNavigationGroup() {
  const isMobile = useIsMobile();
  const { isRouteActive } = useSidebarActiveRoute();

  if (!isMobile) {
    return null;
  }

  return (
    <SidebarGroup>
      <SidebarGroupContent>
        <SidebarMenu>
          <SidebarMenuItem aria-label={t("Home")} title={t("Home")}>
            <SidebarMenuButton asChild isActive={isRouteActive("/")}>
              <Link to="/">
                <HugeiconsIcon icon={HomeIcon} />
                {t("Home")}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem aria-label={t("Search")} title={t("Search")}>
            <SidebarMenuButton
              onClick={() => stores.viewport.actions.toggleCommander(true)}
            >
              <HugeiconsIcon icon={SearchIcon} />
              {t("Search")}
              <KbdGroup className="ml-auto">
                <Kbd>⌘</Kbd>
                <Kbd>K</Kbd>
              </KbdGroup>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  );
}
