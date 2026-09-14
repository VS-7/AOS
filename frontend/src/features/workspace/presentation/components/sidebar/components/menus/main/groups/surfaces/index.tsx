import * as React from "react";
import {
  WindowsNewIcon,
  ArrowRight01Icon,
  Layout01Icon,
  Layers01Icon,
  PlusSignIcon,
  MoreHorizontalIcon,
  PencilIcon,
  Delete01Icon,
} from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { Icon } from "@/components/ui/icon";
import {
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuMotionItem,
  SidebarMenuSub,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAlert } from "@/components/ui/alert-provider";
import { aos } from "@/app/aos";
import { errorMessage } from "@/lib/aos-facade";
import { RenameSurfaceDialog } from "./components/rename-surface-dialog";
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
  id: string;
  key: string;
  label: string;
  icon?: string;
  isActive: boolean;
  /**
   * Whether a person manages this row from here. One a skill brought is
   * removed with the skill, from the marketplace, and deleting it by hand here
   * would leave the skill installed without it.
   */
  managed: boolean;
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
  const { isCurrent: isCurrentView, open: openView, views } = useViews();
  const {
    artifacts,
    current: currentArtifact,
    open: openArtifact,
  } = useArtifacts();

  const rows = React.useMemo(() => {
    const viewRows: SurfaceRow[] = views.map((view) => ({
      kind: "view" as const,
      id: view.id,
      managed: view.scope !== "skill" && !view.skill,
      key: `view:${view.skill ?? ""}:${view.id}`,
      label: view.title,
      icon:
        typeof view.metadata?.icon === "string"
          ? view.metadata.icon
          : undefined,
      // By id, which is what `/views/$id` and the URL carry; the name is only
      // the label. Opening by name landed on "Page not found" — and so did a
      // skill's view opened without its skill.
      isActive: isCurrentView(view),
      onOpen: () => openView(view.id, view.skill),
    }));

    const artifactRows: SurfaceRow[] = artifacts.map((artifact) => ({
      kind: "artifact" as const,
      id: artifact.id,
      managed: !artifact.skill,
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
    isCurrentView,
    openArtifact,
    openView,
    views,
  ]);

  const navigate = useNavigate();
  const { confirm } = useAlert();
  const [renaming, setRenaming] = React.useState<SurfaceRow | null>(null);

  // Views and artifacts had no way out of the sidebar: nothing renamed or
  // removed one, so whatever an agent published stayed there for good.
  async function handleDelete(row: SurfaceRow) {
    const accepted = await confirm({
      title: t("Delete \"{{name}}\"?", { name: row.label }),
      description:
        row.kind === "artifact"
          ? t("The artifact and its files are removed. This action cannot be undone.")
          : t("The view is removed; the collection it shows is not. This action cannot be undone."),
      confirmText: t("Delete"),
      variant: "destructive",
    });
    if (!accepted) return;

    try {
      if (row.kind === "artifact") {
        await aos.client.artifact.delete.mutateOrThrow({ params: { artifact: row.id } });
        for (const tab of aos.stores.viewport.state.tabs.items) {
          if (tab.type === "browser" && tab.metadata?.artifactId === row.id) {
            aos.stores.viewport.actions.closeTab(tab.id);
          }
        }
        await aos.stores.artifact.actions.refresh();
      } else {
        await aos.client.view.delete.mutateOrThrow({ params: { view: row.id } });
        await aos.stores.view.actions.refresh();
        if (row.isActive) void navigate({ to: "/" });
      }
      toast.success(t("Deleted."));
    } catch (error) {
      toast.error(errorMessage(error) ?? t("Unable to delete \"{{name}}\".", { name: row.label }));
    }
  }

  async function handleRename(row: SurfaceRow, name: string) {
    try {
      await aos.client.artifact.update.mutateOrThrow({ params: { artifact: row.id }, body: { name } });
      await aos.stores.artifact.actions.refresh();
      toast.success(t("Renamed."));
    } catch (error) {
      toast.error(errorMessage(error) ?? t("Unable to rename \"{{name}}\".", { name: row.label }));
      throw error;
    }
  }

  return (
    <>
    <RenameSurfaceDialog
      open={renaming != null}
      currentName={renaming?.label ?? ""}
      onOpenChange={(open) => {
        if (!open) setRenaming(null);
      }}
      onRename={(name) => (renaming ? handleRename(renaming, name) : Promise.resolve())}
    />
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
                  {row.managed ? (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <SidebarMenuAction
                          showOnHover
                          aria-label={t("Actions for {{name}}", { name: row.label })}
                        >
                          <HugeiconsIcon icon={MoreHorizontalIcon} />
                        </SidebarMenuAction>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="start" side="right" className="w-40">
                        {row.kind === "artifact" ? (
                          <DropdownMenuItem onClick={() => setRenaming(row)}>
                            <HugeiconsIcon icon={PencilIcon} />
                            {t("Rename")}
                          </DropdownMenuItem>
                        ) : null}
                        <DropdownMenuItem variant="destructive" onClick={() => void handleDelete(row)}>
                          <HugeiconsIcon icon={Delete01Icon} />
                          {t("Delete")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  ) : null}
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
    </>
  );
}
