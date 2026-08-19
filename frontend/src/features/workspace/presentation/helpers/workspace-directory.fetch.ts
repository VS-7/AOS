import { api } from "@/lib/aos-facade";
import type { FractalWorkspaceDirectory } from "@/features/workspace/interfaces/directory.interfaces";

type DirectoryCacheEntry = {
  workspaceId: string;
  directory: FractalWorkspaceDirectory;
  fetchedAt: number;
};

let cache: DirectoryCacheEntry | null = null;
let inflight: Promise<FractalWorkspaceDirectory> | null = null;

const CACHE_TTL_MS = 5_000;

/**
 * Loads the workspace directory once and shares the in-flight promise.
 *
 * Agent store + workspace store both call this during preload so boot does
 * not fan out duplicate HTTP requests.
 *
 * Falls back to raw `fetch` when the typed client schema is stale (missing
 * `workspace.directory`) so Team/identity still boot after gateway restarts.
 *
 * @param workspaceId - Workspace id or `"current"`.
 * @param options - Optional force refresh.
 * @returns Directory roster payload.
 */
export async function loadWorkspaceDirectory(
  workspaceId: string,
  options?: { force?: boolean },
): Promise<FractalWorkspaceDirectory> {
  const id = workspaceId.trim() || "current";
  const now = Date.now();

  if (
    !options?.force &&
    cache &&
    cache.workspaceId === id &&
    now - cache.fetchedAt < CACHE_TTL_MS
  ) {
    return cache.directory;
  }

  if (!options?.force && inflight) {
    return inflight;
  }

  inflight = (async () => {
    const directory = await fetchWorkspaceDirectory(id);

    cache = {
      workspaceId: id,
      directory,
      fetchedAt: Date.now(),
    };

    return directory;
  })().finally(() => {
    inflight = null;
  });

  return inflight;
}

/**
 * Clears the memoized directory cache (after membership/agent mutations).
 */
export function invalidateWorkspaceDirectoryCache(): void {
  cache = null;
  inflight = null;
}

async function fetchWorkspaceDirectory(
  workspaceId: string,
): Promise<FractalWorkspaceDirectory> {
  const empty: FractalWorkspaceDirectory = { users: [], agents: [] };

  try {
    const directoryClient = (
      api.workspace as { directory?: { query: (input: unknown) => Promise<{ data?: unknown }> } }
    ).directory;

    if (directoryClient?.query) {
      const response = await directoryClient.query({
        params: { workspace: workspaceId },
      });
      return unwrapDirectoryPayload(response.data) ?? empty;
    }
  } catch {
    // Fall through to raw fetch when the typed client rejects.
  }

  const response = await fetch(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/directory`,
    { credentials: "include" },
  );

  if (!response.ok) {
    return empty;
  }

  const json = (await response.json()) as { data?: unknown };
  return unwrapDirectoryPayload(json.data) ?? empty;
}

function unwrapDirectoryPayload(
  payload: unknown,
): FractalWorkspaceDirectory | undefined {
  if (!payload || typeof payload !== "object") {
    return undefined;
  }

  if (
    "directory" in payload &&
    payload.directory &&
    typeof payload.directory === "object"
  ) {
    return payload.directory as FractalWorkspaceDirectory;
  }

  if ("users" in payload && "agents" in payload) {
    return payload as FractalWorkspaceDirectory;
  }

  return undefined;
}
