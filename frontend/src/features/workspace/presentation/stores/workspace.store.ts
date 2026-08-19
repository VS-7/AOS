import { AosStore } from "@/app/builders/store";
import {
  FractalWorkspace,
  FractalWorkspaceCreate,
} from "@/features/workspace/interfaces/workspace.interfaces";
import type {
  FractalWorkspaceDirectoryAgent,
  FractalWorkspaceDirectoryUser,
} from "@/features/workspace/interfaces/directory.interfaces";
import { FractalWorkspaceError } from "@/features/workspace/errors/workspace.errors";
import { api } from "@/lib/aos-facade";
import {
  invalidateWorkspaceDirectoryCache,
  loadWorkspaceDirectory,
} from "@/features/workspace/presentation/helpers/workspace-directory.fetch";

const setWorkspaceCookie = (id: string | undefined) => {
  if (typeof window !== "undefined") {
    if (id) {
      document.cookie = `x-workspace-id=${encodeURIComponent(id)}; path=/; max-age=31536000; SameSite=Lax`;
    } else {
      document.cookie = `x-workspace-id=; path=/; max-age=0; SameSite=Lax`;
    }
  }
};

export type WorkspaceContext = FractalWorkspace & { active: boolean };

export interface WorkspaceDirectoryState {
  users: FractalWorkspaceDirectoryUser[];
  agents: FractalWorkspaceDirectoryAgent[];
}

export interface WorkspaceState {
  current: FractalWorkspace | undefined;
  options: WorkspaceContext[];
  /** Member profiles + lean agents with processing chats (not a separate store). */
  directory: WorkspaceDirectoryState;
}

const EMPTY_DIRECTORY: WorkspaceDirectoryState = {
  users: [],
  agents: [],
};

async function fetchDirectoryForWorkspace(
  workspaceId: string | undefined,
  options?: { force?: boolean },
): Promise<WorkspaceDirectoryState> {
  if (!workspaceId) {
    return EMPTY_DIRECTORY;
  }

  const directory = await loadWorkspaceDirectory(workspaceId, options);
  return {
    users: directory.users,
    agents: directory.agents,
  };
}

export const WorkspaceStore = AosStore.create("workspace")
  .withState<WorkspaceState>({
    current: undefined,
    options: [],
    directory: EMPTY_DIRECTORY,
  })
  .withPreload(async (ctx) => {
    // task-12 disclosed divergence: `workspace_list` (Go's
    // `internal/domain/workspace/schema.go` `ListOutput`) answers
    // `{ workspaces, total }`, not a bare array — confirmed live
    // (`POST /api/workspace/list` in the task-12 HTTP exercise). Reading
    // `response.data` directly, as this originally did, hands the object
    // to `.find`/`.map` below and throws on every boot — this store's
    // preload is awaited before the rest of the app initializes
    // (`app/aos.tsx`'s own doc comment), so this was on the critical boot
    // path. Same shape as `activity.list` above: no `wrapOut` fixes an
    // already-named `Output` struct's field, only a call-site read does.
    const response = await api.workspace.list.query();
    const workspacesList: FractalWorkspace[] = (response.data as { workspaces?: FractalWorkspace[] } | undefined)?.workspaces || [];

    let currentWorkspace = ctx.state.get().current;

    // Rebind to the latest object instance from API when ID still exists.
    if (currentWorkspace) {
      const currentWorkspaceId = currentWorkspace.id;
      const latestCurrentWorkspace = workspacesList.find(
        (w) => w.id === currentWorkspaceId,
      );
      currentWorkspace = latestCurrentWorkspace;
    }

    // Default to the first one if none selected or current is gone
    if (!currentWorkspace && workspacesList.length > 0) {
      currentWorkspace = workspacesList[0];
    }

    const options: WorkspaceContext[] = workspacesList.map((w) => ({
      ...w,
      active: currentWorkspace?.id === w.id,
    }));

    // Sync cookie for backend before directory fetch (agent store shares the memo).
    setWorkspaceCookie(currentWorkspace?.id);

    const directory = await fetchDirectoryForWorkspace(currentWorkspace?.id);

    return {
      current: currentWorkspace,
      options,
      directory,
    };
  })
  .withPersistence({
    enabled: true,
  })
  .addAction("refresh", (ctx) => async () => {
    // Same shape fix as `withPreload` above.
    const response = await api.workspace.list.query();
    const workspacesList: FractalWorkspace[] = (response.data as { workspaces?: FractalWorkspace[] } | undefined)?.workspaces || [];

    const state = ctx.state.get();
    let currentWorkspace = state.current;

    // Rebind to the latest object instance from API when ID still exists.
    // Without this, UI can keep stale fields from the previously cached object.
    if (currentWorkspace) {
      const currentWorkspaceId = currentWorkspace.id;
      const latestCurrentWorkspace = workspacesList.find(
        (w) => w.id === currentWorkspaceId,
      );
      currentWorkspace = latestCurrentWorkspace;
    }

    // Default to the first one if none selected or current is gone
    if (!currentWorkspace && workspacesList.length > 0) {
      currentWorkspace = workspacesList[0];
    }

    const options: WorkspaceContext[] = workspacesList.map((w) => ({
      ...w,
      active: currentWorkspace?.id === w.id,
    }));

    setWorkspaceCookie(currentWorkspace?.id);
    invalidateWorkspaceDirectoryCache();
    const directory = await fetchDirectoryForWorkspace(currentWorkspace?.id, {
      force: true,
    });

    ctx.state.set({
      current: currentWorkspace,
      options,
      directory,
    });

    return {
      current: currentWorkspace,
      options,
      directory,
    };
  })
  .addAction("refreshDirectory", (ctx) => async () => {
    const workspaceId = ctx.state.get().current?.id;
    invalidateWorkspaceDirectoryCache();
    const directory = await fetchDirectoryForWorkspace(workspaceId, {
      force: true,
    });
    ctx.state.set({ directory });
    return directory;
  })
  .addAction("switch", (ctx) => async (workspaceId: string) => {
    try {
      // Refresh the list first to ensure we have the latest options
      const { options: refreshedOptions } = await ctx.actions.refresh();
      const workspace = refreshedOptions.find(
        (w: WorkspaceContext) => w.id === workspaceId,
      );

      if (!workspace) {
        return {
          data: null,
          error: new FractalWorkspaceError({
            code: "FRACTAL_WORKSPACE_NOT_FOUND",
          }),
        };
      }

      // Update active status for all options
      const optionsWithActive = refreshedOptions.map((w: WorkspaceContext) => ({
        ...w,
        active: w.id === workspaceId,
      }));

      setWorkspaceCookie(workspace.id);
      invalidateWorkspaceDirectoryCache();
      const directory = await fetchDirectoryForWorkspace(workspace.id, {
        force: true,
      });

      ctx.state.set({
        current: workspace,
        options: optionsWithActive,
        directory,
      });

      return {
        data: workspace,
        error: null,
      };
    } catch (err) {
      return {
        data: null,
        error: err instanceof Error ? err : new Error(String(err)),
      };
    }
  })
  .addAction("create", (ctx) => async (params: FractalWorkspaceCreate) => {
    const response = await api.workspace.create.mutate({ body: params });
    // task-12 disclosed divergence: `workspace_create` answers
    // `CreateOutput { workspace, orchestrator, scaffold, adopted }`
    // (`internal/domain/workspace/schema.go`) — confirmed live via the CLI
    // during the task-12 scratch-state setup — not a bare `Workspace`.
    // Reading `response.data.id` directly, as this originally did, is
    // always `undefined`; the id is at `response.data.workspace.id`. That
    // left `ctx.actions.switch(undefined)` silently failing to select the
    // workspace that was just created.
    const data = (response.data as { workspace?: FractalWorkspace } | undefined)?.workspace;

    if (data) {
      // switch now handles refresh internally and returns { data, error }
      await ctx.actions.switch(data.id);
    }

    return data;
  })
  .addAction("deleteWorkspace", (ctx) => async (workspaceId: string) => {
    const state = ctx.state.get();

    // [Business Rule]: Prevent deletion when no workspace is active
    if (!workspaceId) {
      return {
        data: null,
        error: new FractalWorkspaceError({
          code: "FRACTAL_WORKSPACE_NOT_FOUND",
        }),
      };
    }

    try {
      await api.workspace.delete.mutate({ params: { workspace: workspaceId } });

      // Refresh the list — the deleted workspace is now gone
      // Same shape fix as `withPreload` above.
      const response = await api.workspace.list.query();
      const workspacesList: FractalWorkspace[] = (response.data as { workspaces?: FractalWorkspace[] } | undefined)?.workspaces || [];

      // Determine the next current workspace
      let nextCurrent: FractalWorkspace | undefined;

      if (state.current?.id !== workspaceId) {
        // The deleted workspace was not the active one; keep current
        nextCurrent = state.current;
      } else if (workspacesList.length > 0) {
        // Auto-select the first remaining workspace
        nextCurrent = workspacesList[0];
      }

      const options: WorkspaceContext[] = workspacesList.map((w) => ({
        ...w,
        active: nextCurrent?.id === w.id,
      }));

      setWorkspaceCookie(nextCurrent?.id);
      invalidateWorkspaceDirectoryCache();
      const directory = await fetchDirectoryForWorkspace(nextCurrent?.id, {
        force: true,
      });

      ctx.state.set({
        current: nextCurrent,
        options,
        directory,
      });

      return {
        data: nextCurrent ?? null,
        error: null,
      };
    } catch (err) {
      return {
        data: null,
        error: err instanceof Error ? err : new Error(String(err)),
      };
    }
  })
  .build();
