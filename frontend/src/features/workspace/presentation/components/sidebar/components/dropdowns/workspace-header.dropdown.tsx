import { ChevronsUpDown, Copy, FolderOpen } from "lucide-react";
import { HugeiconsIcon } from "@hugeicons/react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { aos } from "@/app/aos";
import { SETTINGS_SECTIONS } from "@/features/workspace/presentation/components/settings/constants";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";
import { WorkspaceAvatar } from "../shared/workspace-avatar";

const WORKSPACE_SETTINGS_SECTIONS = SETTINGS_SECTIONS.filter(
  (section) => section.group === "workspace",
);

/**
 * Desktop sidebar header dropdown for the active workspace
 * (settings deep-links, Finder, copy ID) — Discord-style layout.
 */
export function AppSidebarWorkspaceHeaderDropdown() {
  const currentWorkspace = aos.stores.workspace.useState((s) => s.current);
  const canRevealInFinder = Boolean(
    typeof window !== "undefined" && window.aos?.system?.showItemInFolder,
  );

  async function handleCopyId() {
    if (!currentWorkspace?.id) return;

    try {
      await navigator.clipboard.writeText(currentWorkspace.id);
      toast.success("Workspace ID copied");
    } catch {
      toast.error("Failed to copy workspace ID");
    }
  }

  async function handleOpenInFinder() {
    if (!currentWorkspace?.path) return;

    const revealed = await window.aos?.system?.showItemInFolder?.(
      currentWorkspace.path,
    );

    if (!revealed) {
      toast.error("Failed to open workspace in Finder");
    }
  }

  return (
    <SidebarMenu className="w-full">
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="default"
              className="h-8 w-full gap-2 px-2 data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <WorkspaceAvatar
                name={currentWorkspace?.name}
                color={currentWorkspace?.color}
                logo={currentWorkspace?.logo}
                className="size-5! rounded-md"
                fallbackClassName="text-[10px] leading-none rounded-md"
              />
              <span className="min-w-0 flex-1 truncate text-sm font-medium">
                {currentWorkspace?.name || "No Workspace"}
              </span>
              <ChevronsUpDown className="ml-auto size-3.5 opacity-60" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="min-w-56"
            align="start"
            side="bottom"
            sideOffset={6}
          >
            <DropdownMenuGroup>
              <DropdownMenuLabel>Settings</DropdownMenuLabel>
              {WORKSPACE_SETTINGS_SECTIONS.map((section) => {
                const navigateArgs =
                  SettingsRouteHelper.sectionIdToNavigateArgs(section.id);

                return (
                  <DropdownMenuItem key={section.id} asChild>
                    <Link to={navigateArgs.to} params={navigateArgs.params}>
                      <span className="flex-1">{section.title}</span>
                      <HugeiconsIcon
                        icon={section.icon}
                        className="size-3.5 text-muted-foreground"
                      />
                    </Link>
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuGroup>

            {canRevealInFinder ? (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="cursor-default"
                  onClick={() => {
                    void handleOpenInFinder();
                  }}
                >
                  <span className="flex-1">Open in Finder</span>
                  <FolderOpen className="size-3.5" />
                </DropdownMenuItem>
              </>
            ) : null}

            <DropdownMenuSeparator />

            <DropdownMenuItem
              className="cursor-default"
              onClick={() => {
                void handleCopyId();
              }}
            >
              <span className="flex-1">Copy Workspace ID</span>
              <Copy className="size-3.5" />
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
