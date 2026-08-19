import { AosApp } from "./builders";
import { api } from "@/lib/aos-facade";
import { stores } from "./stores";
import { triggers } from "./lib/triggers";
import { WorkspaceLayout } from "@/features/workspace/presentation/components/layout";
import { NotFoundComponent } from "@/features/workspace/presentation/pages/not-found/404";

/**
 * The instance the ported pages consume as `aos.page(...)` and
 * `aos.client.task.list.useQuery()`.
 *
 * `client` here is the facade, not `lib/client.ts`: the pages were written
 * against the `client.<feature>.<action>.<method>` shape, and the facade is
 * what presents that.
 *
 * Task 10 update: this now mirrors the pristine `@app/igniter.tsx` shape in
 * full, superseding the Task 9 vertical-slice version (5 stores, one
 * feature's trigger group, the hand-rolled `RootLayout`):
 *
 * - `.withStores(stores, handler)` — the handler explicitly awaits
 *   `stores.workspace.init()` before deriving `namespace.workspaceId` from
 *   it. `AosApp.build()`'s root `beforeLoad` (`app/builders/app.tsx`) runs
 *   this handler *before* its own loop that auto-inits every other
 *   registered store, so `workspace.current` is guaranteed populated by the
 *   time the loop (and later `.withContext(...)` below) run. Without this,
 *   the first render's `stores.namespace.get().workspaceId` would resolve
 *   before `workspace_get` had returned — `WorkspaceLayout`'s own
 *   `namespace`-sync `useEffect` catches that up shortly after, but every
 *   `.withNamespace(...)`-scoped store (`chat`, `activity`, …) would
 *   otherwise start on the wrong (empty) partition for one paint.
 * - `.withContext(...)` — `HomePage`'s loader (`context.workspaces?.
 *   current`, `context.config.user`) and other pristine pages read
 *   `ctx.context.context` (`app/builders/page.ts`'s `beforeLoad`) expecting
 *   this to be populated; unset, it silently defaulted to `{}` (`DefaultContext`
 *   is `Record<string, any>`, so this was a runtime gap, not a type error).
 * - `.withTriggers(triggers)` — the canonical registry (`app/lib/
 *   triggers.ts`, already assembled by earlier tasks) covering every
 *   copied feature's own trigger group, not just `task`'s `tasksGroup`
 *   reached into directly. `aos.triggers.dispatch(...)` now resolves for
 *   any ported feature's triggers, not only tasks'.
 * - `.withLayout(WorkspaceLayout)` — the pristine Fractal shell
 *   (`features/workspace/presentation/components/layout`): full sidebar
 *   navigation, command palette, panels (browser/file/changes/chat), task
 *   dialog. Supersedes `RootLayout` (`app/root-layout.tsx`, now unused),
 *   AOS's own 4-link placeholder shell — without this swap, the 26 ported
 *   features' pages would be reachable only by typing their URL directly,
 *   with no navigation to them anywhere in the UI.
 * - `.withNotFoundComponent(NotFoundComponent)` — the pristine 404 screen
 *   (`features/workspace/presentation/pages/not-found/404.tsx`), wired to
 *   the root route the same way `.withLayout(...)` is (see that method's
 *   doc comment on `AosPage.build()` always parenting to *this instance's*
 *   root route). Previously unset, so TanStack Router's own bare default
 *   would have rendered instead of this screen.
 *
 * `AosPage.build()` (`app/builders/page.ts`) always parents each page's
 * route to *this instance's* root route (built below, inside
 * `AosApp.build()`), never to a route `router.tsx` might create
 * separately — `router.tsx` uses `aos.rootRoute` as *its* own root for the
 * same reason, so there is only ever one root route in the tree.
 */
export const aos = AosApp.create()
  .withClient(api)
  .withStores(stores, async ({ stores }) => {
    await stores.workspace.init();
    await stores.namespace.set((current: Record<string, unknown>) => ({
      ...current,
      workspaceId: stores.workspace.state.current?.id,
    }));
  })
  .withTriggers(triggers)
  .withContext(({ stores }) => ({
    config: stores.config.state,
    workspaces: stores.workspace.state,
  }))
  .withLayout(WorkspaceLayout)
  .withNotFoundComponent(NotFoundComponent)
  .withDefaultPreload("intent")
  .build();
