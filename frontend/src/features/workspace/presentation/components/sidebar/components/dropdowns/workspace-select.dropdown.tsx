import * as React from "react";
import { Check, Plus } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { CreateWorkspaceDialog } from "@/features/workspace/presentation/components/dialogs/upsert";
import { aos } from "@/app/aos";
import { switchWorkspace } from "../shared/switch-workspace";
import { WorkspaceAvatar } from "../shared/workspace-avatar";
import { ArrowLeftRightIcon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import { t } from "@/lib/i18n";

export function AppSidebarWorkspaceSelectDropdown() {
  const { current: currentWorkspace, options: workspaces } =
    aos.stores.workspace.useState();
  const isSuper = aos.stores.auth.useState((state) => state.user?.role === "super");
  // The dialog lives beside the menu, not inside it. Nested in a menu item
  // that refused to close (`onSelect` preventDefault, so the dialog could
  // mount), the menu stayed drawn behind the open dialog and was still open
  // over the new workspace after a successful create.
  const [createOpen, setCreateOpen] = React.useState(false);
  const openingCreate = React.useRef(false);

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              aria-label={t("Switch workspace")}
              className="data-[state=open]:bg-sidebar-accent size-8 w-full data-[state=open]:text-sidebar-accent-foreground"
            >
              <HugeiconsIcon icon={ArrowLeftRightIcon} />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
            align="start"
            side="bottom"
            sideOffset={4}
            onCloseAutoFocus={(event) => {
              // The menu hands focus back to its trigger as it closes; when it
              // closed to open the dialog, that would pull focus out of it.
              if (openingCreate.current) {
                event.preventDefault();
                openingCreate.current = false;
              }
            }}
          >
            {workspaces.map((workspace) => {
              // A workspace record has no "active" flag; the one this window
              // addresses is the store's current, so that is what gets the check.
              const isCurrent = workspace.id === currentWorkspace?.id;
              return (
                <DropdownMenuItem
                  key={workspace.id}
                  onClick={() => {
                    if (!isCurrent) void switchWorkspace(workspace.id);
                  }}
                  aria-current={isCurrent ? "true" : undefined}
                  className="gap-2 p-2 cursor-default"
                >
                  <WorkspaceAvatar
                    name={workspace.name}
                    color={workspace.color}
                    logo={workspace.logo}
                    size="sm"
                    className="rounded-sm"
                    fallbackClassName="rounded-sm"
                  />
                  {workspace.name}
                  {isCurrent ? (
                    <Check className="ml-auto h-4 w-4" aria-hidden />
                  ) : null}
                </DropdownMenuItem>
              );
            })}

            {isSuper ? (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="gap-2 p-2 cursor-default"
                  onSelect={() => {
                    openingCreate.current = true;
                    setCreateOpen(true);
                  }}
                >
                  <div className="flex size-6 items-center justify-center rounded-sm border bg-background">
                    <Plus className="size-4 shrink-0" />
                  </div>
                  {t("New Workspace")}
                </DropdownMenuItem>
              </>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
        {isSuper ? (
          <CreateWorkspaceDialog open={createOpen} onOpenChange={setCreateOpen} />
        ) : null}
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
