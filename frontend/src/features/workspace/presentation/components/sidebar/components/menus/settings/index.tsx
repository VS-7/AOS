"use client";

import {
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  SETTINGS_SECTIONS,
  type SettingsSectionId,
} from "@/features/workspace/presentation/components/settings/constants";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";
import { useSidebarActiveRoute } from "../../../hooks/use-sidebar-active-route";
import { aos } from "@/app/aos";
import { api } from "@/lib/aos-facade";
import type { WorkspaceMember } from "@/features/workspace/interfaces/workspace.interfaces";
import { t } from "@/lib/i18n";

function useSettingsSectionVisibility() {
  const authUser = aos.stores.auth.useState((state) => state.user);
  const workspaceId = aos.stores.workspace.useState((state) => state.current?.id);

  const membersQuery = api.workspace.listMembers.useQuery({
    params: { workspace: workspaceId ?? "" },
    enabled: Boolean(workspaceId),
  });

  const members = (membersQuery.data as WorkspaceMember[] | undefined) ?? [];
  const isWorkspaceOwner = members.some(
    (member) => member.userId === authUser?.id && member.role === "owner",
  );

  const canManageMembers = authUser?.role === "super" || isWorkspaceOwner;
  const canManageUsers = authUser?.role === "super";

  return { canManageMembers, canManageUsers };
}

function isSectionVisible(
  sectionId: SettingsSectionId,
  visibility: { canManageMembers: boolean; canManageUsers: boolean },
): boolean {
  if (sectionId === "user.users") {
    return visibility.canManageUsers;
  }

  if (sectionId === "workspace.members") {
    return visibility.canManageMembers;
  }

  return true;
}

/**
 * Settings navigation menu that replaces the main sidebar while on /settings/*.
 */
export function AppSidebarSettingsMenu() {
  const { isRouteActive } = useSidebarActiveRoute();
  const visibility = useSettingsSectionVisibility();

  function isActive(sectionId: SettingsSectionId): boolean {
    return isRouteActive(SettingsRouteHelper.sectionIdToPath(sectionId), {
      exact: true,
    });
  }

  return (
    <SidebarContent className="scroll-fade scroll-fade-10 gap-0">
      <SidebarGroup>
        <SidebarGroupLabel>{t("User")}</SidebarGroupLabel>
        <SidebarGroupContent>
          <SidebarMenu>
            {SETTINGS_SECTIONS.filter((candidate) => candidate.group === "user")
              .filter((item) => isSectionVisible(item.id, visibility))
              .map((item) => {
                const [group, section] = item.id.split(".");
                return (
                  <SidebarMenuItem key={item.id}>
                    <SidebarMenuButton
                      asChild
                      isActive={isActive(item.id)}
                      tooltip={item.title}
                    >
                      <Link
                        to="/settings/$group/$section"
                        params={{ group, section }}
                      >
                        <HugeiconsIcon icon={item.icon} className="size-4" />
                        <span>{item.title}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>

      <SidebarGroup>
        <SidebarGroupLabel>{t("Workspace")}</SidebarGroupLabel>
        <SidebarGroupContent>
          <SidebarMenu>
            {SETTINGS_SECTIONS.filter(
              (candidate) => candidate.group === "workspace",
            )
              .filter((item) => isSectionVisible(item.id, visibility))
              .map((item) => {
                const [group, section] = item.id.split(".");
                return (
                  <SidebarMenuItem key={item.id}>
                    <SidebarMenuButton
                      asChild
                      isActive={isActive(item.id)}
                      tooltip={item.title}
                    >
                      <Link
                        to="/settings/$group/$section"
                        params={{ group, section }}
                      >
                        <HugeiconsIcon icon={item.icon} className="size-4" />
                        <span>{item.title}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>
  );
}
