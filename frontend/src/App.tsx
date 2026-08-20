import type { JSX } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { DomainError } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";
import { ErrorBoundary } from "@/components/ui/error-boundary";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthGate } from "@/features/auth/AuthGate";
import { router } from "@/app/router";

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

export function App(): JSX.Element {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
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
              <AuthGate>
                <RouterProvider router={router} />
              </AuthGate>
            </TooltipProvider>
          </div>
        </AppStateProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
