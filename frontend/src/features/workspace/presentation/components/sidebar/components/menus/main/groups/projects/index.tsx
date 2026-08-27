import { ChevronRight, Folder } from "lucide-react";
import { useNavigate, useRouterState } from "@tanstack/react-router";

import { Icon } from "@/components/ui/icon";
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuMotionItem,
  SidebarMenuSub,
} from "@/components/ui/sidebar";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { aos } from "@/app/aos";
import { ProjectHelper } from "@/features/project/presentation/helpers/project.helper";
import { HugeiconsIcon } from "@hugeicons/react";
import { AddSquareIcon } from "@hugeicons/core-free-icons";
import { t } from "@/lib/i18n";

function getCurrentProjectId(pathname: string): string | undefined {
  if (!pathname.startsWith("/projects/")) return undefined;
  return decodeURIComponent(
    pathname.replace("/projects/", "").split("/")[0] || "",
  );
}

export function WorkspaceSidebarProjectsGroupMenu() {
  const navigate = useNavigate();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const currentProjectId = getCurrentProjectId(pathname);

  const projects = aos.stores.projects.useState((state) => state.items);

  function openProject(projectId: string) {
    void navigate({ to: "/projects/$id", params: { id: projectId } });
  }

  function handleCreate() {
    void navigate({ to: "/projects/$id", params: { id: "new" } });
  }

  return (
    <Collapsible
      key="projects"
      asChild
      defaultOpen={false}
      className="group/collapsible"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={t("Projects")}>
            <Folder />
            <span>{t("Projects")}</span>
            <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            <SidebarMenuMotionItem index={0}>
              <SidebarMenuItem>
                <SidebarMenuButton onClick={handleCreate}>
                  <HugeiconsIcon
                    icon={AddSquareIcon}
                    className="size-3.5 text-muted-foreground"
                  />
                  <span className="truncate">{t("New project")}</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenuMotionItem>
            {projects.map((project, index) => {
              const isActive = currentProjectId === project.id;
              const iconName = ProjectHelper.getIcon(project.icon);

              return (
                <SidebarMenuMotionItem key={project.id} index={index + 1}>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={isActive}
                      onClick={() => openProject(project.id)}
                    >
                      <Icon
                        value={iconName}
                        fallback="Folder"
                        className="size-3.5 text-muted-foreground rounded-sm"
                      />
                      <span className="truncate">{project.name}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenuMotionItem>
              );
            })}
            {projects.length === 0 ? (
              <span className="px-2 text-xs text-muted-foreground/60">
                {t("No projects yet")}
              </span>
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
}
