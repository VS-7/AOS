import * as React from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  AddSquareIcon,
  ArrowRight01Icon,
  DatabaseIcon,
  Delete01Icon,
  MoreHorizontalIcon,
} from "@hugeicons/core-free-icons";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAlert } from "@/components/ui/alert-provider";
import { errorMessage } from "@/lib/aos-facade";
import type { CollectionDefinition } from "@/features/collection/presentation/helpers/collection-fields.helper";
import { CreateCollectionDialog } from "./components/create-collection-dialog";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";

function getCurrentCollectionId(pathname: string): string | undefined {
  if (!pathname.startsWith("/collections/")) {
    return undefined;
  }

  return decodeURIComponent(
    pathname.replace("/collections/", "").split("/")[0] || "",
  );
}

export function WorkspaceSidebarCollectionsGroupMenu() {
  const navigate = useNavigate();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const currentCollectionId = getCurrentCollectionId(pathname);

  const collections = aos.stores.collections.useState(
    (state) => state.items,
  );

  // By id, the collection's directory name, which is what the page resolves;
  // the name is only the label. Opening by name reached "Page not found", or
  // an empty table for a name differing from the id only in case.
  function openCollection(collectionId: string) {
    void navigate({ to: "/collections/$id", params: { id: collectionId } });
  }

  const { confirm } = useAlert();
  const [creating, setCreating] = React.useState(false);

  async function handleDelete(collection: CollectionDefinition) {
    const accepted = await confirm({
      title: t("Delete \"{{name}}\"?", { name: collection.name }),
      description: t("The collection and every record in it are removed. This action cannot be undone."),
      confirmText: t("Delete"),
      variant: "destructive",
    });
    if (!accepted) return;
    try {
      await aos.client.collection.delete.mutateOrThrow({ params: { collection: collection.id } });
      await aos.stores.collections.actions.refresh();
      if (currentCollectionId === collection.id) void navigate({ to: "/" });
      toast.success(t("Deleted."));
    } catch (error) {
      toast.error(errorMessage(error) ?? t("Unable to delete \"{{name}}\".", { name: collection.name }));
    }
  }

  return (
    <>
    <CreateCollectionDialog open={creating} onOpenChange={setCreating} onCreated={openCollection} />
    <Collapsible
      key="collections"
      asChild
      defaultOpen={false}
      className="group/collapsible"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton tooltip={t("Collections")}>
            <HugeiconsIcon icon={DatabaseIcon} className="size-3.5" />
            <span>{t("Collections")}</span>
            <HugeiconsIcon
              icon={ArrowRight01Icon}
              className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90"
            />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            <SidebarMenuSubItem>
              <SidebarMenuSubButton asChild className="w-full text-muted-foreground hover:text-foreground">
                <button type="button" onClick={() => setCreating(true)}>
                  <HugeiconsIcon icon={AddSquareIcon} className="size-3.5" />
                  <span>{t("New collection")}</span>
                </button>
              </SidebarMenuSubButton>
            </SidebarMenuSubItem>
            {collections.length > 0 ? (
              collections.map((collection) => {
                const isActive = currentCollectionId === collection.id;

                return (
                  <SidebarMenuSubItem key={collection.id}>
                    <SidebarMenuSubButton
                      isActive={isActive}
                      onClick={() => openCollection(collection.id)}
                    >
                      <HugeiconsIcon
                        icon={DatabaseIcon}
                        className="size-3.5 text-muted-foreground"
                      />
                      <span className="truncate">{collection.name}</span>
                    </SidebarMenuSubButton>
                    {/* A skill's collection leaves with the skill. */}
                    {collection.scope !== "skill" ? (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <SidebarMenuAction
                            showOnHover
                            aria-label={t("Actions for {{name}}", { name: collection.name })}
                          >
                            <HugeiconsIcon icon={MoreHorizontalIcon} />
                          </SidebarMenuAction>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start" side="right" className="w-40">
                          <DropdownMenuItem variant="destructive" onClick={() => void handleDelete(collection)}>
                            <HugeiconsIcon icon={Delete01Icon} />
                            {t("Delete")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    ) : null}
                  </SidebarMenuSubItem>
                );
              })
            ) : (
              <span className="px-2 text-xs text-muted-foreground/60">
                {t("No collections yet")}
              </span>
            )}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
    </>
  );
}
