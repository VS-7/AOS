import type { JSX } from "react";
import {
  createRoute,
  createRouter,
  redirect,
  type ErrorComponentProps,
} from "@tanstack/react-router";
import { client } from "@/lib/client";
import { ChatContent } from "@/features/chat/presentation/components/chat-content";
import { Failure } from "@/components/Failure";
import { TasksPage } from "@/features/task/presentation/pages/(main)";
import { TaskDetailsPage } from "@/features/task/presentation/pages/($id)";
import { MemoryGraph } from "@/features/memory/MemoryGraph";
import { FilesPage } from "@/features/file/FilesPage";
import { aos } from "@/app/aos";

/**
 * The router, ported from the original's `@tanstack/react-router` route tree
 * (@app/router.tsx) with the Igniter builder stripped out: `IgniterPage` only
 * ever turned into a plain `createRoute` with a `beforeLoad`/`loader` pair
 * bound to Igniter's RPC client. There is no Igniter backend here, and every
 * page already fetches its own data with TanStack Query against
 * `client.invoke` — a router loader would just be a second place doing the
 * same fetch. See docs/06 - Frontend/React 19 e Bindings.md.
 *
 * `rootRoute` is `aos.rootRoute`, not a second `createRootRoute(...)` this
 * file builds itself — see `app/aos.tsx`'s doc comment on `.withLayout(...)`
 * for why: `aos.page(...)` (what `TasksPage`/`TaskDetailsPage` are built
 * with) always parents its route to the `aos` instance's own root, so this
 * file has to use that same object as its actual root for the tree to be
 * one connected router rather than two, with pages silently rendering
 * outside the app shell. `RootLayout` moved to `app/root-layout.tsx` for
 * the same reason: `aos.tsx` needs it before `router.tsx` can even define
 * `rootRoute`, so it can't live here.
 */
const rootRoute = aos.rootRoute;

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

const routeTree = rootRoute.addChildren([
  indexRoute,
  chatRoute,
  TasksPage,
  TaskDetailsPage,
  memoriesRoute,
  filesRoute,
]);

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
