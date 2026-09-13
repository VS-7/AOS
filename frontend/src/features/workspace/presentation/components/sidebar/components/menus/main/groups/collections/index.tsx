import * as React from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import { ArrowRight01Icon, DatabaseIcon } from "@hugeicons/core-free-icons";
import { useNavigate, useRouterState } from "@tanstack/react-router";

import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { CollectionStore } from "@/features/collection/presentation/stores/collection.store";
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

  // By id, the collection directory name, which is what the page resolves;
  // the name is only the label. Opening by name reached "Page not found", or
  // an empty table for a name differing from the id only in case.
  function openCollection(collectionId: string) {
    void navigate({ to: "/collections/$id", params: { id: collectionId } });
  }

  return (
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
  );
}
