import { Check, ChevronsUpDown, Plus } from "lucide-react";
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

export function AppSidebarWorkspaceSelectDropdown() {
  const { current: currentWorkspace, options: workspaces } =
    aos.stores.workspace.useState();
  const isSuper = aos.stores.auth.useState((state) => state.user?.role === "super");

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
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
          >
            {workspaces.map((workspace) => (
              <DropdownMenuItem
                key={workspace.id}
                onClick={() => {
                  void switchWorkspace(workspace.id);
                }}
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
                {workspace.active ? (
                  <Check className="ml-auto h-4 w-4" />
                ) : null}
              </DropdownMenuItem>
            ))}

            <DropdownMenuSeparator />

            {isSuper ? (
              <CreateWorkspaceDialog
                trigger={
                  <DropdownMenuItem
                    className="gap-2 p-2 cursor-default"
                    onSelect={(e) => e.preventDefault()}
                  >
                    <div className="flex size-6 items-center justify-center rounded-sm border bg-background">
                      <Plus className="size-4 shrink-0" />
                    </div>
                    New Workspace
                  </DropdownMenuItem>
                }
              />
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
