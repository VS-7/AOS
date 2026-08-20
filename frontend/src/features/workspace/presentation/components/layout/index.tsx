import { SidebarProvider } from "@/components/ui/sidebar";
import { WorkspaceSidebar } from "../sidebar";
import { ThemeProvider } from "@/components/ui/theme-provider";
import { WorkspaceCommander } from "../dialogs/commander";
import { TaskDialog } from "../../../../task/presentation/components/dialogs/create";
import { AlertProvider } from "@/components/ui/alert-provider";
import { stores } from "@/app/lib/stores";
import { cn } from "@/lib/utils";
import { Outlet, useRouter, useRouterState } from "@tanstack/react-router";
import { useEffect, type CSSProperties } from "react";
import { WorkspaceNavigation } from "@/features/workspace/presentation/components/navigation";
import { BrowserPanel } from "@/features/workspace/presentation/components/panels/browser";
import { FilePanel } from "@/features/file/presentation/components/panels/files";
import { ChangesPanel } from "@/features/file/presentation/components/panels/changes";
import { ChatPanel } from "@/features/chat/presentation/components/panels/chat";
import { useRealtime } from "@/hooks/use-realtime";
import { aos } from "@/app/aos";
import { useNotification } from "@/hooks/use-notification";
import { useIsMobile } from "@/hooks/use-mobile";
import { WorkspaceNavControlsShell } from "../sidebar/components/workspace-nav-controls-shell";

export function WorkspaceLayout() {
  const sidebar = stores.viewport.useState(
    (state) => state.layout.sidebar.visible,
  );
  const router = useRouter();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const isMobile = useIsMobile();

  const activeTabId = stores.viewport.useState((state) => state.tabs.current);
  const workspaceId = stores.workspace.useState((state) => state.current?.id);
  const tabs = stores.viewport.useState((state) => state.tabs.items ?? []);

  const activeTab = tabs.find((t) => t.id === activeTabId);
  const isBrowserActive = activeTab?.type === "browser";
  const isFileActive = activeTab?.type === "file";
  const isChangesActive = activeTab?.type === "changes";
  const isChatActive = activeTab?.type === "chat";
  const { notify } = useNotification();

  // In-app route changes (Tasks, Goals, …) reveal the Outlet by focusing the
  // "aos" tab. Sidebar chat opens call openChatTab without changing
  // pathname, so chat multi-tabs are not reset by this effect.
  useEffect(() => {
    if (activeTabId === "aos") return;

    stores.viewport.actions.setActiveTab("aos");
  }, [pathname]);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      const currentNamespace = stores.namespace.get();

      if (currentNamespace.workspaceId === workspaceId) {
        return;
      }

      await stores.namespace.set((current) => ({
        ...current,
        workspaceId,
      }));

      if (!cancelled) {
        await router.invalidate();
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [router, workspaceId]);

  useRealtime(
    "activity:created",
    async (event) => {
      await notify(event);
      await router.invalidate();

      aos.stores.activity.actions.refresh();
    },
    [],
  );

  const { setProcessing } = aos.stores.agent.useActions();

  // Listen for real-time processing events
  useRealtime(
    "chat:start-processing",
    (payload) => {
      setProcessing(payload.chatId, payload.agentId, true);
    },
    [],
  );

  useRealtime(
    "chat:end-processing",
    (payload) => {
      setProcessing(payload.chatId, payload.agentId, false);
    },
    [],
  );

  // Streaming snapshots (payload.message) are applied locally by the chat
  // panel via useChat — a global route invalidation on every tool transition
  // would re-render the whole workspace. Only structural events (send,
  // completion, clear, reaction) require a full reconciliation.
  useRealtime("chat:refresh", async (payload) => {
    if (!payload.message) {
      void router.invalidate();
    }
  });

  const routesWithoutLayout = ["/onboarding", "/login"];
  if (routesWithoutLayout.some((route) => pathname.startsWith(route))) {
    return (
      <ThemeProvider>
        <AlertProvider>
          <Outlet />
        </AlertProvider>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider>
      <AlertProvider>
        <SidebarProvider
          open={sidebar}
          onOpenChange={(visibility) =>
            stores.viewport.actions.toggle("layout.sidebar.visible", visibility)
          }
          style={
            {
              "--workspace-rail-width": '0px',
            } as CSSProperties
          }
        >
          <WorkspaceNavControlsShell />
          <WorkspaceSidebar />
          <WorkspaceCommander />
          <div
            className={cn(
              "relative h-screen w-full min-h-0 bg-background shadow-xs grid grid-rows-[auto_1fr] border-l border-y",
            )}
          >
            <header>
              <WorkspaceNavigation />
            </header>
            <main className="flex min-h-0 flex-1 flex-col overflow-hidden min-w-0">
              <div
                className={cn(
                  "flex-1 h-full min-h-0 min-w-0 overflow-hidden",
                  "[&>div]:first:rounded-t-none",
                  "[&>div]:first:rounded-r-none [&>div]:first:rounded-b-none",
                  "[&>form>div]:first:rounded-r-none",
                  (isBrowserActive ||
                    isFileActive ||
                    isChangesActive ||
                    isChatActive) &&
                  "hidden",
                )}
              >
                <Outlet />
              </div>

              <BrowserPanel />
              <FilePanel />
              <ChangesPanel />
              <ChatPanel />
            </main>
          </div>
          <TaskDialog />
        </SidebarProvider>
      </AlertProvider>
    </ThemeProvider>
  );
}
