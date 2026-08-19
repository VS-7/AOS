/**
 * There is no AOS Go backend for this domain yet, but a real, checkable
 * declaration exists in the old Fractal server:
 * `v401/server/src/features/workspace/services/workspace/workspace.service.ts`'s
 * `getDirectory()` — the exact object-construction site for
 * `FractalWorkspaceDirectoryUser`/`Agent` (the type-only
 * `interfaces/directory.interfaces.ts` it imports from was itself erased
 * by the bundler, same as everywhere else, so this construction site is
 * the best available ground truth). Recovered, not guessed — this
 * replaces an earlier pass reconstructed from frontend usage alone
 * (`task`'s `assignee.helper.ts`), which had `username`/`email` backwards
 * as optional (the real construction always sets both), invented a
 * `role` field the real payload never sets, and had `orchestrator`/
 * `processing` backwards as optional (both are always present — the
 * server does `Boolean(agent.orchestrator)` and
 * `processingByAgent.get(agent.id) ?? []`, never omitting either).
 *
 * `FractalWorkspaceDirectoryAgentProcessing`'s shape comes from
 * `FractalChatKindHelper.index_processing_by_agent`'s return type
 * (`chat-kind.helper.ts`, already verified byte-identical between
 * `v401/web` and `v401/server`): `{ chatId, title, kind }`. `kind` stays
 * `string` rather than importing `chat`'s `ChatKind`/`FractalChatKind` to
 * avoid a cross-feature import — always valid, since `FractalChatKind` is
 * a string-literal union.
 */
export interface FractalWorkspaceDirectoryUser {
  id: string;
  name: string;
  username: string;
  email: string;
  image?: string;
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
  orchestrator: boolean;
  processing: FractalWorkspaceDirectoryAgentProcessing[];
}

/** Workspace roster snapshot: `stores.workspace.directory` (viewer-relative — self excluded). */
export interface FractalWorkspaceDirectory {
  users: FractalWorkspaceDirectoryUser[];
  agents: FractalWorkspaceDirectoryAgent[];
}
