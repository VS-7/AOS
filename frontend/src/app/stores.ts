import { AosStore } from "./builders/store";
import { client, rememberedWorkspace, setWorkspace } from "@/lib/client";
import {
  AUTHENTICATED_EVENT,
  SIGNED_OUT_EVENT,
  session,
  status,
  login,
  logout,
  updateProfile,
  changePassword,
} from "@/lib/auth";
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

/**
 * Finds which workspace this window is for, and tells the transport about
 * it — the step that was missing, and that every workspace-scoped screen
 * silently depended on.
 *
 * `workspace_get` names no workspace of its own: it answers for whichever
 * one the *request* addresses, via the `x-workspace-id` header or cookie
 * (`internal/transport/httpapi/middleware.go`'s `ambientIdentity`), and
 * refuses with `AOS_WORKSPACE_NOT_FOUND` — "no workspace was named and
 * none is active" — when the request carries neither. The desktop window
 * sets that up out of band, before the interface ever loads:
 * `cmd/aos-desktop`'s `introspectWorkspace` registers the directory it
 * was launched in and calls `daemonclient.SetWorkspace` with the id, so
 * every Wails-transport call already carries it. Nothing does the
 * equivalent for the HTTP transport, so calling `workspace_get` straight
 * out of this preload 404'd on every boot outside the desktop window. The
 * store then kept its empty default and the header rendered "No
 * Workspace" — with `namespace.workspaceId` left `undefined`, which is
 * also the partition key every `.withNamespace(...)` store resolves
 * against (`app/aos.tsx`).
 *
 * `workspace_list` is the one workspace command that needs no such
 * context, so it goes first: it establishes the id, `setWorkspace`
 * publishes it to the HTTP transport (`lib/client.ts`'s `workspaceHeader`)
 * and to the daemon's cookie, and only then is `workspace_get` a question
 * with an answer. Its entries are already complete `Workspace` objects —
 * same fields `workspace_get` returns — so the list doubles as the
 * `options` the workspace switcher shows, which was hardcoded empty
 * before.
 *
 * Returns `null` when the daemon knows of no workspace at all, which is a
 * real state on a fresh installation (nothing registered yet) and not an
 * error to surface: the caller keeps the empty default.
 */
async function resolveWorkspaces(
  preferredId: string | undefined,
): Promise<{ current: CurrentWorkspaceState; options: CurrentWorkspaceState[] } | null> {
  // With nothing remembered, the workspace the request already addresses: in
  // the desktop window, the one the window adopted for the directory it was
  // launched in. The list is sorted by id, so its first entry was simply the
  // alphabetically first workspace — a fresh window opened "Harness Two"
  // instead of the one it was started in. Asked only when there is no
  // remembered choice, and a refusal (a browser tab with no active workspace)
  // just leaves the first entry as the answer.
  let adoptedId: string | undefined;
  if (!preferredId) {
    try {
      const adopted = (await client.invoke("workspace_get", {
        _reasoning: "finding the workspace this window was opened for, before choosing one to address",
      })) as { id?: string } | undefined;
      adoptedId = adopted?.id;
    } catch {
      adoptedId = undefined;
    }
  }

  const listed = (await client.invoke("workspace_list", {
    _reasoning: "resolving which workspace this window addresses, before any workspace-scoped call",
  })) as { workspaces?: CurrentWorkspaceState[] } | undefined;

  const workspaces = listed?.workspaces ?? [];
  if (workspaces.length === 0) return null;

  const current =
    workspaces.find((workspace) => workspace.id === (preferredId || undefined)) ??
    workspaces.find((workspace) => workspace.id === adoptedId) ??
    workspaces[0];

  // Awaited: inside the desktop window this is what points the bridge at the
  // workspace, and a scoped call made before it lands addresses whichever one
  // the Go side had adopted on its own. A failure is said, not swallowed —
  // but it does not stop the store, whose list entry is already an answer.
  try {
    await setWorkspace(current.id);
  } catch (err) {
    console.error(`[workspace] the window could not be pointed at ${current.id}`, err);
  }
  if (typeof document !== "undefined") {
    document.cookie = `x-workspace-id=${encodeURIComponent(current.id)}; path=/; max-age=31536000; SameSite=Lax`;
  }

  // Now ask for the full record, by id. The list entry is already complete
  // today; this keeps working if `workspace_get` ever returns more than
  // `workspace_list` does, and costs one call. Naming the workspace, rather
  // than leaving it to whatever the request's header says, is what keeps the
  // snapshot from belonging to a different workspace than the data: the
  // answer used to depend on the order two concurrent bridge calls arrived in.
  let detail = current;
  try {
    detail = ((await client.invoke("workspace_get", {
      workspace: current.id,
      _reasoning: "populating the workspace store's current-workspace snapshot (task-type taxonomy, name) at app start",
    })) as CurrentWorkspaceState) ?? current;
  } catch {
    // The list entry is a complete answer on its own — keep it.
  }

  const resolved: CurrentWorkspaceState = { ...detail, tasks: detail.tasks ?? [] };
  return {
    current: resolved,
    options: workspaces.map((workspace) =>
      workspace.id === resolved.id ? resolved : { ...workspace, tasks: workspace.tasks ?? [] },
    ),
  };
}

/**
 * The failure a store action hands back, as a real `Error`.
 *
 * These actions return their failure rather than throw it, and they used to
 * return it as a plain `{message}` object. Callers that throw it on — the
 * account settings forms do, to reach `useForm`'s error path — then handed
 * `useForm` something that is not an `Error`, which it turns into one with
 * `String(err)`: the toast read "[object Object]" instead of "an account
 * needs a name". Keeping the original `Error` also keeps its `code`.
 */
function asError(err: unknown, fallback: string): Error {
  return err instanceof Error ? err : new Error(fallback);
}

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
      // The remembered id, not just whatever this store already holds —
      // which at boot is `null`, so the preload always fell through to
      // `workspaces[0]` and a switched workspace reverted on every reload.
      const resolved = await resolveWorkspaces(
        ctx.state.get().current?.id ?? rememberedWorkspace() ?? undefined,
      );
      if (!resolved) return ctx.state.get();
      return { ...ctx.state.get(), current: resolved.current, options: resolved.options };
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
          const resolved = await resolveWorkspaces(ctx.state.get().current?.id);
          if (!resolved) return;
          ctx.state.set((state) => ({
            ...state,
            current: resolved.current,
            options: resolved.options,
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
    (ctx) =>
      /**
       * Points every subsequent call at another workspace.
       *
       * This was a stub that returned "isn't wired up in this build yet",
       * written when `options` was always empty and there was genuinely
       * nothing to switch to. `workspace_list` fills that list now, so the
       * stub was the only thing left refusing — including on the path
       * `CreateWorkspaceDialog` takes right after creating one, which is
       * how making a workspace ended in an error toast about switching to
       * it.
       *
       * The id is published the same three ways the preload publishes it —
       * the transport's header, the daemon's cookie, and this store — so a
       * caller that reloads (see `switchWorkspace`) comes back addressing
       * the workspace it asked for.
       */
      async (workspaceId: string) => {
        const known = ctx.state.get().options.find((w) => w.id === workspaceId);
        if (!known) {
          return { error: new Error(`No workspace ${workspaceId}.`) };
        }
        try {
          await setWorkspace(workspaceId);
        } catch (err) {
          console.error(`[workspace] the window could not be pointed at ${workspaceId}`, err);
        }
        if (typeof document !== "undefined") {
          document.cookie = `x-workspace-id=${encodeURIComponent(workspaceId)}; path=/; max-age=31536000; SameSite=Lax`;
        }
        try {
          const resolved = await resolveWorkspaces(workspaceId);
          if (resolved) {
            ctx.state.set((state) => ({
              ...state,
              current: resolved.current,
              options: resolved.options,
            }));
          }
          return { error: undefined as Error | undefined };
        } catch (err) {
          return { error: asError(err, "Could not switch workspace.") };
        }
      },
  )
  .addAction(
    "deleteWorkspace",
    (ctx) =>
      /**
       * Unregisters a workspace. `workspace_delete` is a real command (it
       * forgets the registration; the directory on disk is left alone) —
       * this used to refuse before asking.
       */
      async (workspaceId: string) => {
        try {
          // `workspace`, not `id`: that is what workspace_delete's own
          // DeleteInput names it (internal/domain/workspace/schema.go), and
          // the `as never` cast below is what let the wrong name through the
          // typechecker. Sending `id` meant the required field was absent,
          // the command was refused by validation — and the catch below
          // reported the refusal while the store went on to re-read the list
          // and show the workspace still there.
          await client.invoke("workspace_delete", {
            workspace: workspaceId,
            _reasoning: "the person asked to remove this workspace from the installation",
          });
        } catch (err) {
          return { error: asError(err, "Could not delete workspace.") };
        }
        try {
          const resolved = await resolveWorkspaces(
            ctx.state.get().current?.id === workspaceId ? undefined : ctx.state.get().current?.id,
          );
          ctx.state.set((state) => ({
            ...state,
            current: resolved?.current ?? null,
            options: resolved?.options ?? [],
          }));
        } catch {
          // The delete landed; a failed re-read is a stale list, not a
          // failed operation.
        }
        return { error: undefined as Error | undefined };
      },
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
          return { error: undefined as Error | undefined };
        } catch (err) {
          return { error: asError(err, "Login failed.") };
        }
      },
  )
  .addAction(
    "logout",
    (ctx) =>
      /**
       * Real, same reasoning as `login` above — backed by `lib/auth.ts`.
       *
       * The store is cleared and AuthGate told before anything navigates.
       * This used to end the session and leave `isAuthenticated` true, so the
       * account menu's navigation to /login was bounced back to / by
       * `workspace.middleware.ts`, and every home loader fired without a
       * credential before a failed call finally sent the gate to Login — with
       * the URL left at /.
       */
      async () => {
        await logout();
        ctx.state.set((state) => ({ ...state, isAuthenticated: false, user: null }));
        if (typeof window !== "undefined") window.dispatchEvent(new Event(SIGNED_OUT_EVENT));
      },
  )
  .addAction(
    "signedIn",
    (ctx) =>
      /**
       * AuthGate saw somebody sign in again after Login or Onboarding — see
       * `AUTHENTICATED_EVENT`. Set synchronously, before the router remounts
       * under the gate and reads it; the account itself follows.
       */
      async () => {
        ctx.state.set((state) => ({ ...state, isAuthenticated: true, onboarding: "done" as const }));
        try {
          const { user } = await session();
          ctx.state.set((state) => ({ ...state, user: user as unknown as AuthSelfProfile }));
        } catch {
          // The name in the sidebar stays empty until the next read; the
          // session itself is fine, which is what the router needs.
        }
      },
  )
  .addAction(
    "updateProfile",
    (ctx) =>
      /**
       * Real since `POST /api/auth/profile` exists (see
       * `internal/domain/auth`'s `UpdateProfile`). It was a stub, which is
       * why the account settings page could show a name and never change
       * one.
       */
      async (params: { name: string; email: string; image?: string }) => {
        try {
          const { user } = await updateProfile(params.name, params.email);
          ctx.state.set((state) => ({
            ...state,
            user: { ...(state.user ?? {}), ...user } as unknown as AuthSelfProfile,
          }));
          return { error: undefined as Error | undefined };
        } catch (err) {
          return { error: asError(err, "Could not save the profile.") };
        }
      },
  )
  .addAction(
    "updatePassword",
    () =>
      /** Real, backed by `lib/auth.ts`'s `changePassword`. */
      async (params: { currentPassword: string; newPassword: string }) => {
        try {
          await changePassword(params.currentPassword, params.newPassword);
          return { error: undefined as Error | undefined };
        } catch (err) {
          return { error: asError(err, "Could not change the password.") };
        }
      },
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
        error: new Error("API token regeneration isn't wired up in this build yet."),
      }),
  )
  .build();

// The gate is the one that sees a sign-in through its own Login page; the
// store the router reads learns of it here. See `signedIn` above.
if (typeof window !== "undefined") {
  window.addEventListener(AUTHENTICATED_EVENT, () => {
    void authStore.actions.signedIn();
  });
}

/**
 * Reads a list command into a store, tolerating the two shapes the daemon
 * answers with: `{<key>: [...]}` for a wrapped list and a bare array for the
 * commands that answer without a wrapper.
 *
 * Returns the previous value on failure rather than emptying the list: a
 * transient error should not make a populated sidebar look like an empty
 * workspace.
 */
async function loadList<T>(key: "projects_list" | "goals_list", field: string, previous: T[]): Promise<T[]> {
  try {
    const answer = (await client.invoke(key, {
      _reasoning: "refreshing the sidebar list after it changed",
    } as never)) as Record<string, unknown> | T[] | undefined;
    if (Array.isArray(answer)) return answer as T[];
    const items = (answer as Record<string, unknown>)?.[field];
    return Array.isArray(items) ? (items as T[]) : previous;
  } catch {
    return previous;
  }
}

/**
 * These two used to be `async () => {}` — deliberately, because
 * `project.list`/`goal.list` were dormant when they were written. They are
 * registered commands now (`command-map.ts` maps both), so the no-op was the
 * only thing left: the project and goal pages call `refresh()` after every
 * create, update and delete, and nothing ever happened. Both stores also
 * preload now, so the sidebar has something to show before anyone edits
 * anything.
 */
const projectsStore = AosStore.create("projects")
  .withState({
    items: [] as Project[],
  })
  .withPreload(async (ctx) => ({
    ...ctx.state.get(),
    items: await loadList<Project>("projects_list", "projects", ctx.state.get().items),
  }))
  .addAction(
    "refresh",
    (ctx) => async () => {
      const items = await loadList<Project>("projects_list", "projects", ctx.state.get().items);
      ctx.state.set((state) => ({ ...state, items }));
    },
  )
  .build();

const goalsStore = AosStore.create("goals")
  .withState({
    items: [] as Goal[],
  })
  .withPreload(async (ctx) => ({
    ...ctx.state.get(),
    items: await loadList<Goal>("goals_list", "goals", ctx.state.get().items),
  }))
  .addAction(
    "refresh",
    (ctx) => async () => {
      const items = await loadList<Goal>("goals_list", "goals", ctx.state.get().items);
      ctx.state.set((state) => ({ ...state, items }));
    },
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
