/**
 * Reconstructed from usage: the file was type-only and the bundler erased
 * it. There is no Go backend for this domain yet — when there is, this
 * contract becomes verifiable and should be checked against it.
 *
 * `FractalWorkspaceDirectoryUser`/`FractalWorkspaceDirectoryAgent`'s base
 * fields (id/name/username/email/image/role) come from how `task`'s
 * assignee.helper.ts reads them (normalizeUser/findUser) — this is what
 * the previous, now-superseded stub covered. `FractalWorkspaceDirectory`
 * (the container) and the agent's `description`/`orchestrator`/
 * `processing` fields are additions from v401/web/src's real consumers,
 * found while reconstructing this batch:
 * `features/workspace/presentation/helpers/workspace-directory.fetch.ts`
 * (the `{ users, agents }` container shape) and
 * `.../sidebar/.../groups/chat/components/chat-team-list.tsx` (agent
 * `description`/`orchestrator`, coerced into `FractalAgent`, and
 * `processing`, read as `FractalWorkspaceDirectoryAgentProcessing[]`).
 */
export interface FractalWorkspaceDirectoryUser {
  id: string;
  name: string;
  username?: string;
  email?: string;
  image?: string;
  role?: string;
}

/** One live chat this agent is currently processing (sidebar Team tab badge). */
export interface FractalWorkspaceDirectoryAgentProcessing {
  chatId: string;
  title: string;
  kind: string;
}

export interface FractalWorkspaceDirectoryAgent {
  id: string;
  name: string;
  image?: string;
  role?: string;
  description?: string;
  orchestrator?: boolean;
  processing?: FractalWorkspaceDirectoryAgentProcessing[];
}

/** Workspace roster snapshot: `stores.workspace.directory` (viewer-relative — self excluded). */
export interface FractalWorkspaceDirectory {
  users: FractalWorkspaceDirectoryUser[];
  agents: FractalWorkspaceDirectoryAgent[];
}
