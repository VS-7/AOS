import { useNavigate } from "@tanstack/react-router";
import { HugeiconsIcon } from "@hugeicons/react";
import { Logout01Icon, Settings02Icon } from "@hugeicons/core-free-icons";

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { Button } from "@/components/ui/button";
import { aos } from "@/app/aos";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

/**
 * Sidebar footer account chip: avatar + name/email, settings gear, and
 * a dropdown for community links + logout (workspace-select style).
 */
export function AppSidebarUserAccountDropdown({
  isSettingsActive = false,
}: {
  isSettingsActive?: boolean;
}) {
  const navigate = useNavigate();
  const user = aos.stores.auth.useState((state) => state.user);

  const displayName = user?.name?.trim() || "Account";
  const displayEmail = user?.email?.trim() || "";
  const initial = (displayName[0] || displayEmail[0] || "U").toUpperCase();

  async function handleLogout() {
    await aos.stores.auth.actions.logout();
    void navigate({ to: "/login" });
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <div
          className={cn(
            "flex w-full items-center gap-1 rounded-md border border-transparent bg-transparent px-2 py-1",
            "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            // Radix puts data-state=open on the trigger; lift the active chip
            // background to this wrapper via :has (same pattern as sidebar.tsx).
            "has-data-[state=open]:bg-sidebar-accent has-data-[state=open]:text-sidebar-accent-foreground",
          )}
        >
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton className="h-auto min-w-0 flex-1 gap-2 p-0 hover:bg-transparent active:bg-transparent data-[state=open]:hover:bg-transparent">
                <Avatar size="sm" className="size-8 rounded-md">
                  {user?.image ? (
                    <AvatarImage
                      src={user.image}
                      alt={displayName}
                      className="rounded-md"
                    />
                  ) : null}
                  <AvatarFallback className="rounded-md text-xs font-medium">
                    {initial}
                  </AvatarFallback>
                </Avatar>

                <div className="min-w-0 flex-1 text-left leading-tight">
                  <p className="truncate text-sm font-medium text-foreground">
                    {displayName}
                  </p>
                </div>
              </SidebarMenuButton>
            </DropdownMenuTrigger>

            <DropdownMenuContent
              className="min-w-56 rounded-lg"
              align="start"
              side="top"
              sideOffset={8}
            >
              <DropdownMenuItem
                className="cursor-default gap-2 text-destructive focus:text-destructive"
                onClick={() => {
                  void handleLogout();
                }}
              >
                <HugeiconsIcon icon={Logout01Icon} className="size-3.5" />
                {t("Logout")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              "size-8 shrink-0 text-muted-foreground hover:bg-transparent hover:text-foreground",
              isSettingsActive && "text-foreground",
            )}
            aria-label={t("Open settings")}
            onClick={() => {
              aos.stores.viewport.actions.openSettings("user.general");
            }}
          >
            <HugeiconsIcon icon={Settings02Icon} className="size-3.5" />
          </Button>
        </div>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
