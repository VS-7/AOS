import { AosStore } from "./builders/store";
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
 * shim. Task 10 almost certainly replaces every store here with a real one
 * backed by actual Go commands — `workspace`/`projects`/`goals` start
 * empty because there is no `workspace.directory`, `project.list`, or
 * `goal.list` Go command yet (see `lib/command-map.ts`: `project.*` and
 * `goal.*` are dormant).
 */

const workspaceStore = AosStore.create("workspace")
  .withState({
    directory: {
      users: [] as FractalWorkspaceDirectoryUser[],
      agents: [] as FractalWorkspaceDirectoryAgent[],
    },
    current: null as { id: string; name: string; tasks: WorkspaceTaskType[] } | null,
  })
  .build();

const authStore = AosStore.create("auth")
  .withState({
    user: null as AuthSelfProfile | null,
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
