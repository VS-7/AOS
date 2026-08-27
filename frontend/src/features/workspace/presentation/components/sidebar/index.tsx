import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  Sidebar,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { AppSidebarFilesMenu } from "./components/menus/files";
import { AppSidebarSettingsMenu } from "./components/menus/settings";
import { stores } from "@/app/lib/stores";
import type { WorkspaceSidebarMenu } from "@/features/workspace/presentation/stores/viewport.store";
import { ArrowLeftIcon } from "lucide-react";
import { WorkspaceSidebarMainMenu } from "./components/menus/main";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { AppSidebarUserAccountDropdown } from "./components/dropdowns/user-account.dropdown";
import { t } from "@/lib/i18n";

function getMenuRank(menu: WorkspaceSidebarMenu) {
  if (menu === "files") return 1;
  if (menu === "settings") return 2;
  return 0;
}

function resolveMenuTitle(menu: WorkspaceSidebarMenu) {
  if (menu === "files") return "Files";
  if (menu === "settings") return "Settings";
  return "Chats";
}

export function WorkspaceSidebar() {
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const activeTab = stores.viewport.useState((state) =>
    state.tabs.items.find((tab) => tab.id === state.tabs.current),
  );
  const activeTabId = activeTab?.id;
  const activeTabType = activeTab?.type;
  const rawMenu = stores.viewport.useState(
    (state) => state.layout.sidebar.menu,
  ) as WorkspaceSidebarMenu;

  const [lastFileTabId, setLastFileTabId] = React.useState<string | null>(null);
  const [previousMenu, setPreviousMenu] =
    React.useState<WorkspaceSidebarMenu>(rawMenu);

  const activeMenu = rawMenu;
  const direction =
    getMenuRank(activeMenu) >= getMenuRank(previousMenu) ? 1 : -1;
  const isOnSettingsRoute = pathname.startsWith("/settings");

  React.useEffect(() => {
    if (activeMenu !== previousMenu) {
      setPreviousMenu(activeMenu);
    }
  }, [activeMenu, previousMenu]);

  React.useEffect(() => {
    if (isOnSettingsRoute && activeMenu !== "settings") {
      stores.viewport.actions.setSidebarMenu("settings");
      return;
    }

    if (!isOnSettingsRoute && activeMenu === "settings") {
      stores.viewport.actions.setSidebarMenu("main");
    }
  }, [isOnSettingsRoute, activeMenu]);

  React.useEffect(() => {
    if (
      activeTabType === "file" &&
      activeTabId &&
      activeTabId !== lastFileTabId
    ) {
      stores.viewport.actions.setSidebarMenu("files");
      setLastFileTabId(activeTabId);
    } else if (activeTabType !== "file") {
      setLastFileTabId(null);
    }
  }, [activeTabType, activeTabId, lastFileTabId]);

  function handleBackToWorkspace() {
    stores.viewport.actions.setSidebarMenu("main");
    if (isOnSettingsRoute) {
      void navigate({ to: "/" });
    }
  }

  return (
    <Sidebar>
      <SidebarHeader
        className="h-14 shrink-0 border-sidebar-border"
        aria-hidden
      />

      {activeMenu !== "main" && (
        <AnimatePresence mode="wait" initial={false} custom={direction}>
          <motion.div
            key={`back-${activeMenu}`}
            custom={direction}
            initial={{
              opacity: 0,
              x: direction > 0 ? 28 : -28,
              filter: "blur(10px)",
            }}
            animate={{ opacity: 1, x: 0, filter: "blur(0px)" }}
            exit={{
              opacity: 0,
              x: direction > 0 ? -22 : 22,
              filter: "blur(8px)",
            }}
            transition={{ duration: 0.24, ease: "easeOut" }}
            className="mt-1"
          >
            <SidebarGroup>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton onClick={handleBackToWorkspace}>
                      <ArrowLeftIcon />
                      {t("Back to workspace")}
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </motion.div>
        </AnimatePresence>
      )}

      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        <AnimatePresence mode="wait" initial={false} custom={direction}>
          <motion.div
            key={activeMenu}
            custom={direction}
            initial={{
              opacity: 0,
              x: direction > 0 ? 28 : -28,
              filter: "blur(10px)",
            }}
            animate={{ opacity: 1, x: 0, filter: "blur(0px)" }}
            exit={{
              opacity: 0,
              x: direction > 0 ? -22 : 22,
              filter: "blur(8px)",
            }}
            transition={{ duration: 0.24, ease: "easeOut" }}
            className={
              activeMenu === "files"
                ? "absolute inset-0 flex min-h-0 flex-col"
                : "absolute inset-0 flex min-h-0 flex-col"
            }
            aria-label={resolveMenuTitle(activeMenu)}
          >
            {activeMenu === "files" ? <AppSidebarFilesMenu /> : null}
            {activeMenu === "settings" ? <AppSidebarSettingsMenu /> : null}
            {activeMenu === "main" ? <WorkspaceSidebarMainMenu /> : null}
          </motion.div>
        </AnimatePresence>
      </div>

      {activeMenu === "main" ? (
        <SidebarFooter>
          <AppSidebarUserAccountDropdown
            isSettingsActive={isOnSettingsRoute}
          />
        </SidebarFooter>
      ) : null}
    </Sidebar>
  );
}
