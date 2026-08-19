import type { JSX } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { DomainError } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";
import { ErrorBoundary } from "@/components/ui/error-boundary";
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
          <AuthGate>
            <RouterProvider router={router} />
          </AuthGate>
        </AppStateProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
