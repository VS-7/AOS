import { AosStore } from "./builders/store";
import { client } from "@/lib/client";
import { session } from "@/lib/auth";
import type {
  FractalWorkspaceDirectoryAgent,
  FractalWorkspaceDirectoryUser,
} from "@/features/workspace/interfaces/directory.interfaces";
import type { AuthSelfProfile } from "@/features/auth/presentation/stores/auth.store";
import type { FractalProject } from "@/features/project/interfaces/project.interfaces";
import type { FractalGoal } from "@/features/goal/interfaces/goal.interfaces";

/** Workspace-level task-type taxonomy entry (`currentWorkspace.tasks`), read by the filter bar and kanban/list cards to render a type's label/color. Shape fixed by that usage, not guessed. */
interface WorkspaceTaskType {
  id: string;
  label?: string;
  color?: string;
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
 *   part of the ported Fractal frontend, so it has no Fractal call-name to
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
      users: [] as FractalWorkspaceDirectoryUser[],
      agents: [] as FractalWorkspaceDirectoryAgent[],
    },
    current: null as { id: string; name: string; tasks: WorkspaceTaskType[] } | null,
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
  .build();

const authStore = AosStore.create("auth")
  .withState({
    user: null as AuthSelfProfile | null,
  })
  .withPreload(async (ctx) => {
    try {
      const { user } = await session();
      return { ...ctx.state.get(), user };
    } catch {
      return ctx.state.get();
    }
  })
  .build();

const projectsStore = AosStore.create("projects")
  .withState({
    items: [] as FractalProject[],
  })
  .build();

const goalsStore = AosStore.create("goals")
  .withState({
    items: [] as FractalGoal[],
  })
  .build();

interface ViewportState {
  page: {
    sidebar: { visible: boolean };
    details: { visible: boolean };
  };
  tasks: {
    // `dialogs/create/index.tsx` reads `state.tasks.dialog.visible` but
    // wrote to the path `"tasks.dialog"` (flat) — a pre-existing
    // inconsistency in the source, not introduced by this port. Modeled as
    // nested here (matching the read, the more central of the two) and the
    // two write call sites were corrected to `"tasks.dialog.visible"`.
    dialog: { visible: boolean };
  };
}

function getAtPath(state: unknown, segments: string[]): unknown {
  return segments.reduce<unknown>(
    (acc, key) => (acc != null && typeof acc === "object" ? (acc as Record<string, unknown>)[key] : undefined),
    state,
  );
}

function setAtPath(segments: string[], value: unknown): unknown {
  if (segments.length === 0) return value;
  return { [segments[0]!]: setAtPath(segments.slice(1), value) };
}

const viewportStore = AosStore.create("viewport")
  .withState<ViewportState>({
    page: { sidebar: { visible: true }, details: { visible: true } },
    tasks: { dialog: { visible: false } },
  })
  .addAction(
    "toggle",
    (ctx) =>
      /**
       * Generic dotted-path boolean toggle — the ported code calls this
       * with paths its store never had a schema for (`"tasks.dialog"`,
       * `"tasks.dialog.visible"`, `"page.details.visible"`), so this stays
       * shape-agnostic rather than switching on known keys.
       */
      (path: string, value?: boolean) => {
        const segments = path.split(".");
        const current = getAtPath(ctx.state.get(), segments);
        const next = typeof value === "boolean" ? value : !current;
        ctx.state.set(setAtPath(segments, next) as never);
      },
  )
  .addAction(
    "createTab",
    () =>
      /**
       * AOS has no multi-tab side panel (Fractal's original opened
       * attachments/browser links into one). No-op until that UX exists —
       * same category as `openChatTab`.
       */
      (_tab: { title: string; url: string; type: string }) => {
        // Intentionally inert — see doc comment above.
      },
  )
  .build();

export const stores = {
  workspace: workspaceStore,
  auth: authStore,
  projects: projectsStore,
  goals: goalsStore,
  viewport: viewportStore,
};
