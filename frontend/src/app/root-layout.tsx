import { useEffect, useState } from "react";
import type { JSX } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Outlet, useRouterState } from "@tanstack/react-router";
import { client, isDesktop, setWorkspace } from "@/lib/client";
import { logout } from "@/lib/auth";
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
import { Failure } from "@/components/Failure";
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
 * The app shell: sidebar navigation, theme picker, connection status, and
 * the routed page.
 *
 * Lives here, not in `router.tsx`, so `app/aos.tsx` can pass it to
 * `AosApp`'s `.withLayout(...)` — the app instance's root route needs this
 * component at construction time (`createRootRouteWithContext<...>()
 * ({component: ...})`, inside `app/builders/app.tsx`'s `build()`), and
 * `router.tsx` needs that already-built `aos.rootRoute` to exist before it
 * can build its own route tree (see `router.tsx`'s own comment on why it
 * uses `aos.rootRoute` directly instead of creating a second root route).
 * If `RootLayout` stayed in `router.tsx`, `aos.tsx` importing it would
 * create `aos.tsx` → `router.tsx` → `aos.tsx`, a real cycle.
 */
export function RootLayout(): JSX.Element {
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
          <button
            type="button"
            className="status"
            onClick={() => {
              // A hard reload, deliberately — the same shape as the
              // original's own auth-state transitions (see OnboardingForm's
              // init step). AuthGate lives above this whole tree, so it is
              // what re-checks status() and shows LoginPage next; there is
              // no lighter-weight way to hand control back to it from here.
              void logout().finally(() => window.location.reload());
            }}
          >
            Sign out
          </button>
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
