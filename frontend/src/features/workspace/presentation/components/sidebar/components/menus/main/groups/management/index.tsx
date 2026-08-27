import * as React from "react";
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  CheckListIcon,
  RepeatIcon,
  TargetIcon,
  FolderIcon,
  Store01Icon,
} from "@hugeicons/core-free-icons";
import { stores } from "@/app/lib/stores";
import { WorkspaceSidebarCollectionsGroupMenu } from "../collections";
import { WorkspaceSidebarProjectsGroupMenu } from "../projects";
import { WorkspaceSidebarSurfacesGroupMenu } from "../surfaces";
import { useSidebarActiveRoute } from "../../../../../hooks/use-sidebar-active-route";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

export function WorkspaceSidebarManagementGroup() {
  const { isRouteActive } = useSidebarActiveRoute();
  const isFilesMenuActive = aos.stores.viewport.useState(
    (state) => state.layout.sidebar.menu === "files",
  );

  return (
    <SidebarGroup>
      <SidebarGroupContent>
        <SidebarMenu>
          <SidebarMenuItem aria-label={t("Tasks")} title={t("Tasks")}>
            <SidebarMenuButton asChild isActive={isRouteActive("/tasks")}>
              <Link to="/tasks">
                <HugeiconsIcon icon={CheckListIcon} />
                {t("Tasks")}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem aria-label={t("Routines")} title={t("Routines")}>
            <SidebarMenuButton asChild isActive={isRouteActive("/routines")}>
              <Link to="/routines">
                <HugeiconsIcon icon={RepeatIcon} />
                {t("Routines")}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem aria-label={t("Goals")} title={t("Goals")}>
            <SidebarMenuButton asChild isActive={isRouteActive("/goals")}>
              <Link to="/goals">
                <HugeiconsIcon icon={TargetIcon} />
                {t("Goals")}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem aria-label={t("Files")} title={t("Files")}>
            <SidebarMenuButton
              isActive={isFilesMenuActive}
              onClick={() => stores.viewport.actions.setSidebarMenu("files")}
            >
              <HugeiconsIcon icon={FolderIcon} />
              {t("Files")}
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem aria-label={t("Marketplace")} title={t("Marketplace")}>
            <SidebarMenuButton asChild isActive={isRouteActive("/marketplace")}>
              <Link to="/marketplace">
                <HugeiconsIcon icon={Store01Icon} />
                {t("Marketplace")}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <WorkspaceSidebarProjectsGroupMenu />
          <WorkspaceSidebarSurfacesGroupMenu />
          <WorkspaceSidebarCollectionsGroupMenu />
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  );
}
