import * as React from "react";
import {
  WindowsNewIcon,
  ArrowRight01Icon,
  Layout01Icon,
  Layers01Icon,
  PlusSignIcon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";

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
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import { useArtifacts } from "@/features/artifact/presentation/hooks/use-artifacts";
import { CreateArtifactDialog } from "@/features/artifact/presentation/components/create-artifact-dialog";
import { useViews } from "@/features/view/presentation/hooks/use-views";
import { t } from "@/lib/i18n";

type SurfaceRow = {
  kind: "view" | "artifact";
  key: string;
  label: string;
  icon?: string;
  isActive: boolean;
  onOpen: () => void;
};

/**
 * Unified Surfaces collapsible — views and artifacts in one sidebar group.
 *
 * Keeps feature open semantics: views navigate to `/views/$id`, artifacts open
 * as browser tabs via {@link useArtifacts}. Row icons stay dynamic via `Icon`;
 * chrome / kind cues use HugeIcons.
 */
export function WorkspaceSidebarSurfacesGroupMenu() {
  const { current: currentView, open: openView, views } = useViews();
  const {
    artifacts,
    current: currentArtifact,
    open: openArtifact,
  } = useArtifacts();

  const rows = React.useMemo(() => {
    const viewRows: SurfaceRow[] = views.map((view) => ({
      kind: "view" as const,
      key: `view:${view.id}`,
      label: view.title,
      icon:
        typeof view.metadata?.icon === "string"
          ? view.metadata.icon
          : undefined,
      isActive: currentView === view.name,
      onOpen: () => openView(view.name),
    }));

    const artifactRows: SurfaceRow[] = artifacts.map((artifact) => ({
      kind: "artifact" as const,
      key: `artifact:${artifact.id}`,
      label: artifact.name,
      icon: ArtifactHelper.getIcon(),
      isActive: currentArtifact === artifact.id,
      onOpen: () => openArtifact(artifact),
    }));

    return [...viewRows, ...artifactRows].sort((left, right) =>
      left.label.localeCompare(right.label),
    );
  }, [
    artifacts,
    currentArtifact,
    currentView,
    openArtifact,
    openView,
    views,
  ]);

  return (
    <Collapsible
      key="surfaces"
      asChild
      defaultOpen={false}
      className="group/collapsible"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={t("Surfaces")}>
            <HugeiconsIcon icon={Layers01Icon} />
            <span>{t("Surfaces")}</span>
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90"
            />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            <SidebarMenuItem>
              <CreateArtifactDialog>
                <SidebarMenuButton className="text-muted-foreground hover:text-foreground">
                  <HugeiconsIcon icon={PlusSignIcon} className="size-3.5" />
                  <span>{t("New artifact")}</span>
                </SidebarMenuButton>
              </CreateArtifactDialog>
            </SidebarMenuItem>
            {rows.map((row, index) => (
              <SidebarMenuMotionItem key={row.key} index={index}>
                <SidebarMenuItem>
                  <SidebarMenuButton isActive={row.isActive} onClick={row.onOpen}>
                    <Icon
                      value={row.icon}
                      fallback={row.kind === "view" ? "BarChart3" : "AppWindow"}
                      className="size-3.5 text-muted-foreground"
                    />
                    <span className="truncate flex-1">{row.label}</span>
                    {row.kind === "view" ? (
                      <HugeiconsIcon
                        icon={Layout01Icon}
                        className="size-3 shrink-0 text-muted-foreground/50"
                      />
                    ) : (
                      <HugeiconsIcon
                        icon={WindowsNewIcon}
                        className="size-3 shrink-0 text-muted-foreground/50"
                      />
                    )}
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenuMotionItem>
            ))}
            {rows.length === 0 ? (
              <span className="px-2 text-xs text-muted-foreground/60">
                {t("No surfaces yet")}
              </span>
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
}
