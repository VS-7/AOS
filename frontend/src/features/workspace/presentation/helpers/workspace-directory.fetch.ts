import { api } from "@/lib/aos-facade";
import { users as listAccounts } from "@/lib/auth";
import type {
  WorkspaceDirectory,
  WorkspaceDirectoryAgent,
  WorkspaceDirectoryUser,
} from "@/features/workspace/interfaces/directory.interfaces";

type DirectoryCacheEntry = {
  workspaceId: string;
  directory: WorkspaceDirectory;
  fetchedAt: number;
};

let cache: DirectoryCacheEntry | null = null;
let inflight: Promise<WorkspaceDirectory> | null = null;

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
): Promise<WorkspaceDirectory> {
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

/**
 * Builds the roster from what the daemon actually publishes.
 *
 * There is no `workspace_directory` command, and there never was — the
 * original composed this server-side and AOS's Go registry has no equivalent.
 * What it has is the two halves: `agents_list`, which is live, and
 * `/api/auth/users`, which lists the accounts on the installation.
 *
 * Until this, the call was mapped to `null` and every caller resolved to
 * `{ users: [], agents: [] }` — quietly, with no error anywhere. That emptiness
 * is what the sidebar's Team tab, the task assignee picker, the chat
 * participant list and every avatar in the interface were rendering: the
 * roster was not failing to load, it was loading nothing.
 *
 * A failure in either half is not a failure of the whole. An installation
 * where the account list is unreachable should still show its agents, and the
 * other way round, so each is caught on its own and contributes what it can.
 */
async function fetchWorkspaceDirectory(
  workspaceId: string,
): Promise<WorkspaceDirectory> {
  const [agents, users] = await Promise.all([
    fetchAgents(workspaceId),
    fetchUsers(),
  ]);
  return { users, agents };
}

async function fetchAgents(workspaceId: string): Promise<WorkspaceDirectoryAgent[]> {
  try {
    const response = await api.agent.list.query(
      workspaceId && workspaceId !== "current"
        ? { query: { workspace: workspaceId } }
        : {},
    );
    const raw = (response.data as { agents?: unknown[] } | undefined)?.agents;
    if (!Array.isArray(raw)) return [];

    return raw.map((entry) => {
      const agent = entry as Record<string, unknown>;
      return {
        id: String(agent.id ?? ""),
        name: String(agent.name ?? agent.id ?? ""),
        image: typeof agent.image === "string" ? agent.image : undefined,
        role: typeof agent.role === "string" ? agent.role : undefined,
        description:
          typeof agent.description === "string" ? agent.description : undefined,
        orchestrator: Boolean(agent.orchestrator),
        // Which chats an agent is mid-turn on is realtime state, not a field
        // of the record: `chat:start-processing` / `chat:end-processing` are
        // what carry it, and `WorkspaceLayout` already routes both into the
        // agent store. Reporting an empty list here rather than inventing one
        // leaves that store the single source of it.
        processing: [],
      } satisfies WorkspaceDirectoryAgent;
    });
  } catch {
    return [];
  }
}

async function fetchUsers(): Promise<WorkspaceDirectoryUser[]> {
  try {
    const { users } = await listAccounts();
    return users.map((user) => ({
      id: user.id,
      name: user.name,
      username: user.username,
      email: user.email,
    }));
  } catch {
    return [];
  }
}
