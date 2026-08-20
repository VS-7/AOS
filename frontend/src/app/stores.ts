import { AosStore } from "./builders/store";
import { client } from "@/lib/client";
import { session, status, login, logout } from "@/lib/auth";
import type {
  WorkspaceDirectoryAgent,
  WorkspaceDirectoryUser,
} from "@/features/workspace/interfaces/directory.interfaces";
import type { AuthSelfProfile } from "@/features/auth/presentation/stores/auth.store";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import type { WorkspaceMember } from "@/features/workspace/interfaces/workspace.interfaces";

/**
 * Task 9 additions: the 25 newly-copied features read many more store
 * namespaces (`aos.stores.activity`, `.agent`, `.browser`, `.chat`,
 * `.collections`, `.config`, `.files`, `.theme`) than the 5 above this
 * comment, which is why this file's own original doc comment (below)
 * scoped itself to `task`'s needs and deferred the rest to "Task 10" —
 * before this task's bulk copy actually brought those stores' own
 * `*.store.ts` files in. Each is registered here as its pristine,
 * unmodified export (`AosStore.create(...).build()` in its own feature's
 * `presentation/stores/`) — mechanical wiring, not a rebuild.
 *
 * `viewport` replaces the hand-rolled store below (removed) with the
 * pristine `ViewportStore` (`workspace/presentation/stores/viewport.
 * store.ts`) instead of adding a second store under a new name: it is a
 * verified superset, not a divergent shape — same `page.sidebar.visible`/
 * `page.details.visible`/`tasks.dialog.visible` fields, same generic
 * dotted-path `toggle(path, value?)` action `task`'s three real call
 * sites (`tasks.trigger.ts`, `dialogs/create/index.tsx`, `($id)/index.
 * tsx`) already use, plus a real `createTab` (the hand-rolled one was an
 * inert stub) and the `tabs`/`agent`/`inbox`/`project`/`goal`/`settings`/
 * `commander` fields the newly-copied panels need. Checked against every
 * `aos.stores.viewport` read in `features/task` before swapping — none
 * use a field or action this store doesn't have.
 */
import { ActivityStore } from "@/features/activity/presentation/stores/activity.store";
import { AgentStore } from "@/features/agent/presentation/stores/agent.store";
import { ArtifactStore } from "@/features/artifact/presentation/stores/artifact.store";
import { BrowserStore } from "@/features/workspace/presentation/stores/browser.store";
import { ChatStore } from "@/features/chat/presentation/stores/chat.store";
import { CollectionStore } from "@/features/collection/presentation/stores/collection.store";
import { ConfigStore } from "@/features/config/presentation/stores/config.store";
import { FilesStore } from "@/features/file/presentation/stores/files.store";
import { ThemeStore } from "@/features/theme/presentation/stores/theme.store";
import { ViewStore } from "@/features/view/presentation/stores/view.store";
import { ViewportStore } from "@/features/workspace/presentation/stores/viewport.store";
import { loadWorkspaceDirectory } from "@/features/workspace/presentation/helpers/workspace-directory.fetch";

/** Workspace-level task-type taxonomy entry (`currentWorkspace.tasks`), read by the filter bar and kanban/list cards to render a type's label/color. Shape fixed by that usage, not guessed. */
interface WorkspaceTaskType {
  id: string;
  label?: string;
  color?: string;
}

/**
 * Task 9 addition: `path`/`color`/`logo`/`git`/`worktrees` are read by the
 * freshly-copied workspace header dropdown and settings screens
 * (`git`/`worktrees` sections) but were never set by this store's own
 * `.withPreload` — `workspace_get` doesn't return them yet, so they stay
 * `undefined` in practice, same honest-empty-state policy as `directory`/
 * `projects`/`goals` below. Typed here (not imported from `Workspace`)
 * to keep this state object's own literal shape — the thing `.withPreload`
 * actually constructs — the source of truth, rather than casting to a
 * richer imported type the preload doesn't populate.
 */
interface CurrentWorkspaceState {
  id: string;
  name: string;
  tasks: WorkspaceTaskType[];
  path?: string;
  color?: string;
  logo?: string;
  git?: {
    branchPrefix?: string;
    forcePush?: boolean;
    commitInstructions?: string;
    prInstructions?: string;
  };
  worktrees?: {
    deleteOldWorktrees?: boolean;
    worktreeLimit?: number;
    onCreateScript?: string;
  };
  /**
   * Read by `chat-team-list.tsx`'s sidebar roster. `workspace.listMembers`
   * is dormant (see `command-map.ts`) — always `undefined` here, same
   * honest-empty-state policy as `directory`/`projects`/`goals`.
   */
  members?: WorkspaceMember[];
  /**
   * Read by the workspace-select dropdown to mark the active entry in
   * `options` (always empty — see that field's own doc comment) — `true`
   * for `current` itself when rendered in that same list.
   */
  active?: boolean;
}

/**
 * Not in the brief's file list — added because the copied `task` feature
 * reads five store namespaces (`workspace`, `auth`, `projects`, `goals`,
 * `viewport`) that don't exist without this. The brief scoped `aos.tsx` to
 * zero stores, deferring the full 17-store set to Task 10; this file is the
 * minimum this vertical slice needs to typecheck and render, built with the
 * real `AosStore` builder (`app/builders/store.ts`, already shipped by
 * Tasks 1-5, never previously exercised end-to-end) rather than an ad hoc
 * shim.
 *
 * Two of the five are wired to real data via `.withPreload(...)`, which
 * `AosApp.build()`'s root `beforeLoad` awaits for every store before any
 * page's own loader runs (see `app/builders/app.tsx`):
 *
 * - `auth.user` — AOS's real, already-working session (`lib/auth.ts`'s
 *   `session()`), the same one `<AuthGate>` already established before
 *   this router ever mounts.
 * - `workspace.current` — the real, already-registered `workspace_get`
 *   command, called directly (not through the facade — this store isn't
 *   part of the ported AOS frontend, so it has no AOS call-name to
 *   translate) exactly the way `app/root-layout.tsx`'s own workspace query
 *   already does.
 *
 * `workspace.directory`, `projects`, and `goals` stay empty: there is no
 * Go command to populate them from yet (`command-map.ts` marks
 * `workspace.listMembers`, `project.list`, and `goal.list` all dormant).
 * Faking that data would be worse than an honest empty state, so this
 * stays exactly that until those commands exist.
 */

const workspaceStore = AosStore.create("workspace")
  .withState({
    directory: {
      users: [] as WorkspaceDirectoryUser[],
      agents: [] as WorkspaceDirectoryAgent[],
    },
    current: null as CurrentWorkspaceState | null,
    /**
     * Read by the workspace-select dropdown to list switchable workspaces.
     * AOS is single-workspace today (no `workspace.list` UI beyond this
     * store's own `current`) — always empty, same honest-empty-state
     * policy as `directory`/`projects`/`goals` above.
     */
    options: [] as CurrentWorkspaceState[],
  })
  .withPreload(async (ctx) => {
    try {
      const out = (await client.invoke("workspace_get", {
        _reasoning: "populating the workspace store's current-workspace snapshot (task-type taxonomy, name) at app start",
      })) as { id: string; name: string; tasks?: WorkspaceTaskType[] };
      return {
        ...ctx.state.get(),
        current: { id: out.id, name: out.name, tasks: out.tasks ?? [] },
      };
    } catch {
      // No workspace registered yet, or the daemon isn't reachable — the
      // rest of the app already has its own failure handling for that
      // (see root-layout.tsx's own `workspace.error` rendering); this
      // store just stays at its empty default rather than throwing out of
      // the root beforeLoad, which would take every route down with it.
      return ctx.state.get();
    }
  })
  .addAction(
    "refresh",
    (ctx) =>
      /**
       * Task 9 addition: `settings/workspace/profile/index.tsx` calls
       * this after saving. Re-runs the same `workspace_get` the preload
       * above already uses.
       */
      async () => {
        try {
          const out = (await client.invoke("workspace_get", {
            _reasoning: "refreshing the workspace store's current-workspace snapshot after a settings save",
          })) as { id: string; name: string; tasks?: WorkspaceTaskType[] };
          ctx.state.set((state) => ({
            ...state,
            current: { ...state.current, id: out.id, name: out.name, tasks: out.tasks ?? [] },
          }));
        } catch {
          // Keep the last known snapshot on a transient failure.
        }
      },
  )
  .addAction(
    "refreshDirectory",
    (ctx) =>
      /**
       * Task 9 addition: `use-chat-composer.ts` (pristine copy) calls
       * this when neither prop-supplied nor cached agents are available.
       * Reuses the same `loadWorkspaceDirectory` fetch `agent.store.ts`
       * already calls at preload (real, self-contained — HTTP with a raw-
       * fetch fallback, not gated on a `command-map.ts` entry), forced to
       * bypass its 5s cache.
       */
      async () => {
        const directory = await loadWorkspaceDirectory("current", { force: true });
        ctx.state.set((state) => ({ ...state, directory }));
      },
  )
  .addAction(
    "switch",
    () =>
      /**
       * Task 9 addition, disclosed stub: AOS is single-workspace (see
       * `options` above) — there is nothing to switch to yet. Returns an
       * explicit error rather than silently pretending success.
       */
      async (_workspaceId: string) => ({
        error: { message: "Switching workspaces isn't wired up in this build yet." },
      }),
  )
  .addAction(
    "deleteWorkspace",
    () =>
      /** Disclosed stub — same reasoning as `switch` above. */
      async (_workspaceId: string) => ({
        error: { message: "Deleting a workspace isn't wired up in this build yet." },
      }),
  )
  .build();

const authStore = AosStore.create("auth")
  .withState({
    user: null as AuthSelfProfile | null,
    // Task 9 addition: `workspace.middleware.ts` (freshly copied) reads
    // `isAuthenticated`/`onboarding` to decide `/login` vs `/onboarding`
    // redirects — the same two facts `<AuthGate>` already checks via
    // `lib/auth.ts`'s own `status()` before this router mounts. Populated
    // from that same real source below, not the facade's `session.get`
    // (a different, AOS-shaped call this store deliberately doesn't
    // use — see this file's top doc comment on why `auth`/`workspace`
    // stay on AOS's own integration).
    isAuthenticated: false,
    onboarding: "waiting" as "done" | "waiting",
  })
  .withPreload(async (ctx) => {
    try {
      const [{ user }, authStatus] = await Promise.all([session(), status()]);
      return {
        ...ctx.state.get(),
        // `lib/auth.ts`'s `PublicUser` (AOS's own, real shape) vs
        // `AuthSelfProfile` (the pristine `auth.store.ts`'s richer shape,
        // e.g. `createdAt`) — cast, not a real reconciliation of the two.
        user: user as unknown as AuthSelfProfile,
        isAuthenticated: authStatus.authenticated,
        onboarding: authStatus.onboarded ? "done" : "waiting",
      };
    } catch {
      return ctx.state.get();
    }
  })
  .addAction(
    "login",
    (ctx) =>
      /**
       * Task 9 addition, real (not a stub): the freshly-copied `features/
       * auth/presentation/pages/login/index.tsx` and onboarding steps call
       * this. Backed by `lib/auth.ts`'s own `login` — the same call
       * `<AuthGate>` uses — not the pristine `auth.store.ts`'s facade-based
       * `api.auth.login` version, which manages its own non-HttpOnly
       * `document.cookie` and would fight AOS's real HttpOnly session
       * cookie rather than reuse it.
       */
      async (params: { email: string; password: string }) => {
        try {
          const { user } = await login(params.email, params.password);
          ctx.state.set((state) => ({
            ...state,
            isAuthenticated: true,
            // Cast — same `PublicUser` vs `AuthSelfProfile` gap as the
            // preload above.
            user: { ...user, hasToken: true, tokenMasked: null } as unknown as AuthSelfProfile,
          }));
          return { error: undefined as { message: string } | undefined };
        } catch (err) {
          return { error: { message: err instanceof Error ? err.message : "Login failed." } };
        }
      },
  )
  .addAction(
    "logout",
    () =>
      /** Real, same reasoning as `login` above — backed by `lib/auth.ts`. */
      async () => {
        await logout();
      },
  )
  .addAction(
    "updateProfile",
    () =>
      /**
       * Task 9 addition, disclosed stub: no equivalent in `lib/auth.ts`
       * (AOS's profile-edit flow, if any, is out of this port's scope).
       * Returns an explicit error rather than silently pretending success —
       * see this file's top doc comment on the honest-empty-state policy.
       */
      async (_params: unknown) => ({
        error: { message: "Profile editing isn't wired up in this build yet." },
      }),
  )
  .addAction(
    "updatePassword",
    () =>
      /** Disclosed stub — same reasoning as `updateProfile` above. */
      async (_params: unknown) => ({
        error: { message: "Password change isn't wired up in this build yet." },
      }),
  )
  .addAction(
    "refreshUser",
    (ctx) =>
      /** Real: re-runs the same `session()` the preload above already uses. */
      async () => {
        try {
          const { user } = await session();
          ctx.state.set((state) => ({ ...state, user: user as unknown as AuthSelfProfile }));
        } catch {
          // Keep the last known user rather than clearing it on a transient failure.
        }
      },
  )
  .addAction(
    "regenerateToken",
    () =>
      /** Disclosed stub — same reasoning as `updateProfile` above. */
      async () => ({
        success: false,
        token: undefined as string | undefined,
        error: { message: "API token regeneration isn't wired up in this build yet." },
      }),
  )
  .build();

const projectsStore = AosStore.create("projects")
  .withState({
    items: [] as Project[],
  })
  .addAction(
    "refresh",
    () =>
      /**
       * Task 9 addition, no-op: `project.list` is dormant (`command-map.
       * ts`) — this store never fetches, so there is nothing to
       * re-fetch. The freshly-copied project detail page calls this
       * after create/update/delete mutations; kept callable so those
       * call sites compile without pretending a refetch happens.
       */
      async () => {},
  )
  .build();

const goalsStore = AosStore.create("goals")
  .withState({
    items: [] as Goal[],
  })
  .addAction(
    "refresh",
    () =>
      /** No-op — same reasoning as `projects`'s `refresh` above. */
      async () => {},
  )
  .build();

/**
 * Review round 2 fix: this is now the single canonical store registry.
 *
 * Before this fix, `app/lib/stores.ts` (a pristine `v401/web` copy) built
 * its *own* `AosStore.router({...})` registry from the pristine
 * `WorkspaceStore`/`AuthStore`/etc. singletons — a second source of truth
 * for `workspace` alongside this file's own hand-built `workspaceStore`.
 * `AosApp.build()`'s root `beforeLoad` (`app/builders/app.tsx`) only calls
 * `.init()` on stores reachable from *this* object (the one `app/aos.tsx`
 * passes to `.withStores(...)`) — the pristine `WorkspaceStore` was never
 * initialized by anything, so `features/workspace/presentation/
 * components/layout/index.tsx`'s `stores.workspace.current?.id` (reading
 * the *other* registry via `@/app/lib/stores`) stayed `undefined` forever,
 * even after this file's `workspaceStore` successfully populated its own
 * `current` from `workspace_get`.
 *
 * Fix: `app/lib/stores.ts` now re-exports this exact object instead of
 * building a second one (see that file). Wrapping it in `AosStore.router`
 * here — the same call the pristine registry used — is not just to keep
 * `stores.namespace` working for `layout/index.tsx`'s workspace-switch
 * writer; it also calls `_attachRuntime` on every store here, which none
 * of them had before (`.withStores(...)` alone never does that — see
 * `app/builders/app.tsx`). Every `.withNamespace(...)` config already set
 * on these stores (`workspaceStore`, `chat.store.ts`, `activity.store.ts`,
 * …) was silently resolving against an always-empty namespace map until
 * now (`AosStoreBuilt`'s `_getNamespaces` defaults to `() => ({})`) — a
 * real, positive side effect of unifying the two registries, not a risk:
 * `task`'s already-verified pages never depended on multi-namespace
 * behavior (AOS is single-workspace today), so there is nothing for this
 * to break, only a previously-inert wiring to turn on.
 *
 * Ledger triage, final review: `view`/`artifact` were the *third* instance
 * of exactly this bug — `use-views.ts`/`use-artifacts.ts` read `ViewStore`/
 * `ArtifactStore` (`features/{view,artifact}/presentation/stores/*.store.
 * ts`) as pristine singletons the same way `layout/index.tsx` once read
 * `WorkspaceStore`, never registered here, so `.init()` never ran and
 * their `withPreload` never fetched `view.list`/`artifact.list`. Added
 * below; `AosStoreBuilt.useState()` now also warns loudly (`app/builders/
 * store.ts`) the first time any future store repeats this, independent of
 * which one.
 */
export const stores = AosStore.router({
  prefix: "aos",
  stores: {
    workspace: workspaceStore,
    auth: authStore,
    projects: projectsStore,
    goals: goalsStore,
    artifact: ArtifactStore,
    view: ViewStore,
    viewport: ViewportStore,
    activity: ActivityStore,
    agent: AgentStore,
    browser: BrowserStore,
    chat: ChatStore,
    collections: CollectionStore,
    config: ConfigStore,
    files: FilesStore,
    theme: ThemeStore,
  },
});
