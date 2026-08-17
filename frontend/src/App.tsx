import type { JSX } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { DomainError } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";
import { ErrorBoundary } from "@/components/ui/error-boundary";
import { AuthGate } from "@/features/auth/AuthGate";
import { router } from "@/router";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Realtime invalidates what changed, so polling would only add load.
      // Retrying a domain refusal would repeat a refusal.
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
