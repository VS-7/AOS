import type { JSX } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { DomainError } from "@/lib/client";
import { AppStateProvider } from "@/lib/app-state";
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
    <QueryClientProvider client={queryClient}>
      <AppStateProvider>
        <RouterProvider router={router} />
      </AppStateProvider>
    </QueryClientProvider>
  );
}
