import { useEffect, useState } from "react";
import type { JSX } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  Outlet,
  Link,
  useRouterState,
  type ErrorComponentProps,
} from "@tanstack/react-router";
import { client, isDesktop, setWorkspace } from "@/lib/client";
import { useRealtime } from "@/lib/realtime";
import {
  listThemes,
  resolveAppearance,
  selectTheme,
  storedChoice,
  watchSystemAppearance,
  type AppearancePreference,
  type ThemeSummary,
} from "@/lib/theme";
import { ChatContent } from "@/features/chat/presentation/components/chat-content";
import { Failure } from "@/components/Failure";
import { TaskBoard } from "@/features/task/TaskBoard";
import { MemoryGraph } from "@/features/memory/MemoryGraph";
import { FilesPage } from "@/features/file/FilesPage";
import { ApprovalModal } from "@/components/ApprovalModal";
import {
  SidebarProvider,
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarInset,
  SidebarTrigger,
} from "@/components/ui/sidebar";

/**
 * The router, ported from the original's `@tanstack/react-router` route tree
 * (@app/router.tsx) with the Igniter builder stripped out: `IgniterPage` only
 * ever turned into a plain `createRoute` with a `beforeLoad`/`loader` pair
 * bound to Igniter's RPC client. There is no Igniter backend here, and every
 * page already fetches its own data with TanStack Query against
 * `client.invoke` — a router loader would just be a second place doing the
 * same fetch. See docs/06 - Frontend/React 19 e Bindings.md.
 */
const rootRoute = createRootRoute({ component: RootLayout });

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  loader: async () => {
    const out = (await client.invoke("chats_list", {
      _reasoning: "the application is starting",
    })) as { chats: Array<{ id: string; title: string }> };
    const first = out.chats?.[0];
    if (first) throw redirect({ to: "/chat/$chatId", params: { chatId: first.id } });
    return null;
  },
  component: () => <p className="empty">No conversation exists yet.</p>,
});

const chatRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/chat/$chatId",
  component: () => {
    const { chatId } = chatRoute.useParams();
    return <ChatContent chatId={chatId} />;
  },
});

const tasksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tasks",
  component: () => (
    <>
      <header>
        <h2>Tasks</h2>
        <p className="subtitle">
          Moving a card calls the same command an agent calls, with the same guards behind it.
        </p>
      </header>
      <TaskBoard />
    </>
  ),
});

const memoriesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/memories",
  component: () => (
    <>
      <header>
        <h2>Memories</h2>
        <p className="subtitle">What the orchestrator knows, and how it connects.</p>
      </header>
      <MemoryGraph agent="atlas" />
    </>
  ),
});

const filesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/files",
  component: () => (
    <div className="flex h-full flex-col gap-3">
      <header>
        <h2>Files</h2>
        <p className="subtitle">The same workspace tree a human reaches by SSH, browsable here.</p>
      </header>
      <FilesPage />
    </div>
  ),
});

const routeTree = rootRoute.addChildren([indexRoute, chatRoute, tasksRoute, memoriesRoute, filesRoute]);

/**
 * What a route renders instead of its page when a loader throws — most often
 * the daemon being unreachable. `reset` re-runs the failed route rather than
 * reloading the page, so a person who started the daemon back up doesn't lose
 * their place.
 */
function RouteErrorFallback({ error, reset }: ErrorComponentProps): JSX.Element {
  return (
    <div className="empty" role="alert">
      <Failure error={error} />
      <button type="button" onClick={() => reset()}>
        Try again
      </button>
    </div>
  );
}

export const router = createRouter({ routeTree, defaultErrorComponent: RouteErrorFallback });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

/** The app shell: sidebar navigation, theme picker, connection status, and the routed page. */
function RootLayout(): JSX.Element {
  const queryClient = useQueryClient();
  const connection = useRealtime(queryClient);
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  const workspace = useQuery({
    queryKey: ["workspace"],
    queryFn: async () => {
      const out = (await client.invoke("workspace_get", {
        _reasoning: "the application is starting",
      })) as { id: string; name: string };
      setWorkspace(out.id);
      return out;
    },
  });

  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader>
          <h1 className="px-2 text-sm font-medium tracking-tight">{workspace.data?.name ?? "AOS"}</h1>
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname.startsWith("/chat")}>
                  <Link to="/">Chat</Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname === "/tasks"}>
                  <Link to="/tasks">Tasks</Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname === "/memories"}>
                  <Link to="/memories">Memories</Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname === "/files"}>
                  <Link to="/files">Files</Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter>
          <ThemePicker />
          <span className="status" data-state={connection} aria-live="polite">
            <span className="dot" />
            {connection === "open" ? "connected" : connection}
          </span>
          <span className="status">{isDesktop() ? "desktop" : "browser"}</span>
        </SidebarFooter>
      </Sidebar>

      <SidebarInset>
        <div className="main">
          <SidebarTrigger className="self-start" />
          {workspace.error && <Failure error={workspace.error} />}
          <Outlet />
        </div>
      </SidebarInset>

      <ApprovalModal />
    </SidebarProvider>
  );
}

/** The theme picker, and the listener that keeps `auto` following the system. */
function ThemePicker(): JSX.Element {
  const initial = storedChoice();
  const [theme, setTheme] = useState(initial.theme);
  const [preference, setPreference] = useState<AppearancePreference>(initial.appearance);
  const [themes, setThemes] = useState<ThemeSummary[]>([]);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    listThemes().then(setThemes).catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    selectTheme(theme, preference).catch(() => setFailed(true));
  }, [theme, preference]);

  useEffect(() => {
    // Only while the preference is auto: somebody who chose dark did not ask to
    // be switched to light at sunrise.
    if (preference !== "auto") return;
    return watchSystemAppearance(() => {
      void selectTheme(theme, "auto");
    });
  }, [preference, theme]);

  if (failed) return <span className="status">theme unavailable</span>;

  return (
    <>
      <label className="status">
        Theme
        <select value={theme} onChange={(e) => setTheme(e.target.value)} aria-label="Theme">
          {themes.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </label>
      <label className="status">
        {resolveAppearance(preference)}
        <select
          value={preference}
          onChange={(e) => setPreference(e.target.value as AppearancePreference)}
          aria-label="Appearance"
        >
          <option value="auto">Follow the system</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </label>
    </>
  );
}
