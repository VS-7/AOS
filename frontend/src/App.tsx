import type { JSX } from "react";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { DomainError } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";
import { ErrorBoundary } from "@/components/ui/error-boundary";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthGate } from "@/features/auth/AuthGate";
import { WorkspaceGate } from "@/features/workspace/WorkspaceGate";
import { useRealtime } from "@/lib/realtime";
import { t } from "@/lib/i18n";
import { router } from "@/app/router";
import { I18nProvider, useTranslation } from "@/lib/i18n";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Realtime invalidates what changed, so polling would only add load.
      // Retrying a domain refusal would repeat a refusal.
      //
      // M5 of the final review: checked whether this predicate is actually
      // dead, since every ported query goes through `lib/aos-facade.ts`'s
      // `useQuery` wrapper, which hardcodes `retry: false` per-query (a
      // more specific option always wins over this default) and whose
      // `queryFn` throws a bare `Error`/`EnvelopeError`, never a
      // `DomainError` — for those ~117 call sites, `error instanceof
      // DomainError` is correct but moot, since `retry` never runs at all.
      // It is not dead overall, though: four of AOS's own components call
      // `client.invoke` directly with no facade in between and no local
      // `retry` override — `root-layout.tsx`'s workspace query, `Approval
      // Modal.tsx`, `chat-timeline.tsx`, `avatar.tsx` — and `client.invoke`
      // (`lib/client.ts`, off-limits to this branch) does throw real
      // `DomainError`s. This predicate is what decides retry behavior for
      // those four; the code was correct, the missing piece was this
      // comment saying so.
      refetchOnWindowFocus: false,
      retry: (count, error) => !(error instanceof DomainError) && count < 2,
    },
  },
});

/**
 * Opens the one connection to the daemon's event channel.
 *
 * It has to be a component rather than a call in `App`: the hook needs the
 * `QueryClient` from context, which only exists below `QueryClientProvider`.
 *
 * Nothing mounted this. `lib/realtime.ts`'s own doc describes it as "the one
 * WebSocket the app opens", and it used to be opened by `app/root-layout.tsx`
 * — which was deleted when `WorkspaceLayout` superseded it (see `app/aos.tsx`),
 * taking the connection with it and leaving no import of this module anywhere.
 * The listener side survived intact: `hooks/use-realtime.ts` and its eight
 * call sites went on subscribing through `onRealtimeEvent` to a registry that
 * nothing ever fed. So every live update in the application — an answer being
 * written, a task moving, a file changing — silently did nothing, and the only
 * way to see that anything had happened was to reload the page.
 */
function RealtimeConnection(): JSX.Element | null {
  const state = useRealtime(useQueryClient());
  // "reconnecting" here means the daemon stopped answering, not that a socket
  // is retrying: the desktop process reports its health (see lib/realtime.ts).
  // Saying so beats what the application did before — going on rendering its
  // screens while every action failed with an untranslated "Load failed", and
  // no way back short of relaunching.
  if (state !== "reconnecting") return null;
  return <DaemonUnreachableBanner />;
}

/**
 * A line across the top, not a blocking overlay.
 *
 * What is already on screen was read from a daemon that was answering, so it
 * is still worth looking at; what is not worth doing is a write that will
 * fail. The banner says which of the two the window is in and gets out of the
 * way when the daemon comes back.
 */
function DaemonUnreachableBanner(): JSX.Element {
  return (
    <div
      role="status"
      className="fixed inset-x-0 top-0 z-50 bg-destructive px-4 py-1.5 text-center text-xs font-medium text-destructive-foreground"
    >
      {t("The daemon is not answering. Reconnecting…")}
    </div>
  );
}

/**
 * Re-renders the whole interface when the language changes.
 *
 * Most call sites translate through the module-level `t` rather than a hook —
 * 958 strings across 224 files, many of them in toasts, helpers and constant
 * tables that are not components and cannot hold one. That function reads the
 * active locale at call time, so the strings are already correct; what it
 * cannot do is tell React that a string it returned earlier is now stale.
 * Keying the tree on the locale is what does: switching language remounts
 * once, and every `t(...)` in the tree runs again.
 */
function Localized({ children }: { children: JSX.Element }): JSX.Element {
  const { locale } = useTranslation();
  return <div key={locale} className="contents">{children}</div>;
}

export function App(): JSX.Element {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <RealtimeConnection />
        <I18nProvider>
        <AppStateProvider>
          {/*
            * The shell wrapper, `Toaster` and `TooltipProvider` mirror the
            * original's own `@app/app.tsx`, which this file otherwise
            * diverged from — each was missing, and each was a silent failure
            * rather than a visible one:
            *
            * - `Toaster` renders nothing until it is mounted, and `toast.*` is
            *   called 321 times across 68 ported files. Every one of them was
            *   a no-op.
            * - `Tooltip` here does not wrap itself in a provider (see
            *   components/ui/tooltip.tsx), and Radix's `Tooltip.Root` throws
            *   without one in scope. 19 files render a bare `<Tooltip>`.
            * - `text-xs` is the original's base type size for the whole
            *   interface. Without it the tree inherited the 13px `body`
            *   size ThemeProvider sets, which made every screen render about
            *   8% larger than the design it was ported from.
            *
            * A `div`, not the original's `main`: `WorkspaceLayout` already
            * renders a `main` inside this, and nesting one in another is
            * invalid. Nothing here is styling that depends on the tag.
            */}
          <div className="bg-background text-foreground text-xs min-h-screen">
            <Toaster position="top-center" />
            <TooltipProvider>
              <Localized>
                <AuthGate>
                  <WorkspaceGate>
                    <RouterProvider router={router} />
                  </WorkspaceGate>
                </AuthGate>
              </Localized>
            </TooltipProvider>
          </div>
        </AppStateProvider>
        </I18nProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
