import { aos } from "@/app/aos";
import { t } from "@/lib/i18n";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

export interface OpenChatTabParams {
  chatId: string;
  title?: string;
}

/**
 * Reads the chat id stored on a viewport tab, if any.
 *
 * @param tab - Viewport tab (or metadata-bearing slice).
 * @returns Chat id when the tab is a chat surface.
 */
export function getTabChatId(tab: {
  metadata?: Record<string, string | number | boolean>;
}): string | null {
  return typeof tab.metadata?.chatId === "string" ? tab.metadata.chatId : null;
}

/**
 * Resolves the active chat from the current viewport tab.
 *
 * Prefer this over pathname `/chats/$id` when the sidebar opens chats as
 * workspace tabs — the route may stay on `/` / management pages.
 *
 * @param activeTab - Currently focused viewport tab.
 * @returns Chat id when a chat tab is active.
 */
export function resolveActiveChatId(
  activeTab: ViewportTabState | undefined | null,
): string | undefined {
  if (!activeTab || activeTab.type !== "chat") {
    return undefined;
  }

  return getTabChatId(activeTab) ?? undefined;
}

/**
 * Finds an existing private agent DM chat id from the local chat list.
 *
 * Matches participant agent rows, or legacy slug-as-id chats during migration.
 *
 * @param chats - Chats already loaded in the client store.
 * @param agentId - Agent slug.
 * @returns Matching chat id when present.
 */
export function findAgentDmChatId(
  chats: Array<Pick<Chat, "id" | "kind" | "participants">>,
  agentId: string,
): string | undefined {
  const match = chats.find((chat) => {
    if (chat.id === agentId) {
      return true;
    }
    if (chat.kind && chat.kind !== "dm") {
      return false;
    }
    return (chat.participants ?? []).some(
      (participant) =>
        participant.type === "agent" && participant.id === agentId,
    );
  });
  return match?.id;
}

/**
 * Opens (or focuses) a chat viewport tab for the given chat id.
 * Dedupes by chat id so the same thread reuses one tab.
 *
 * @param params - Chat id and optional tab title.
 * @returns The focused or newly created tab id.
 *
 * @example
 * ```typescript
 * openChatTab({ chatId: "550e8400-…", title: "Atlas" });
 * ```
 */
export function openChatTab(params: OpenChatTabParams): string {
  const { chatId, title } = params;
  const tabs = aos.stores.viewport.state.tabs.items;

  const existing = tabs.find(
    (tab) => tab.type === "chat" && getTabChatId(tab) === chatId,
  );

  if (existing) {
    if (title && existing.title !== title) {
      aos.stores.viewport.actions.updateTab(existing.id, { title });
    }
    aos.stores.viewport.actions.setActiveTab(existing.id);
    return existing.id;
  }

  const createdTabId = aos.stores.viewport.actions.createTab({
    type: "chat",
    title: title ?? chatId,
    closable: true,
    url: undefined,
    metadata: {
      chatId,
    },
  });

  if (createdTabId) {
    aos.stores.viewport.actions.setActiveTab(createdTabId);
  }

  return createdTabId;
}

/**
 * Finds or creates a private user↔agent DM, then opens it as a viewport tab.
 *
 * There is no `chat.findOrCreateDm` command on the Go side — the backend
 * has `chats_list`/`chats_create` only (internal/domain/chat/commands.go),
 * no dedicated find-or-create. This used to call `chat.findOrCreateDm`
 * anyway; `command-map.ts` declares that path `null` (no Go counterpart),
 * so every call fell straight to the dormant-domain error path and threw
 * "Unable to open agent DM." on every attempt — this is why the DM
 * button never worked. `findAgentDmChatId` right above already existed for
 * exactly this search and had no caller.
 *
 * @param params - Agent slug and optional title.
 * @returns The focused or newly created tab id.
 */
export async function openAgentDmTab(params: {
  agentId: string;
  title?: string;
}): Promise<string> {
  const listResponse = await aos.client.chat.list.query({
    query: { kind: "dm" },
  });
  // A list that could not be read is not an empty list: treating it as one
  // went on to create a second DM with someone who already had one.
  if (listResponse.error) throw listResponse.error;
  const chats = (listResponse?.data?.chats ?? []) as Chat[];
  const existingId = findAgentDmChatId(chats, params.agentId);
  if (existingId) {
    const existing = chats.find((chat) => chat.id === existingId);
    return openChatTab({
      chatId: existingId,
      title: params.title ?? existing?.title ?? params.agentId,
    });
  }

  const createResponse = await aos.client.chat.create.mutate({
    body: {
      title: params.title ?? params.agentId,
      kind: "dm",
      participants: [{ type: "agent", id: params.agentId }],
    },
  });
  const chat = (
    createResponse as { data?: { chat?: Chat }; error?: unknown } | null | undefined
  )?.data?.chat;
  // The refusal itself, when there is one: callers toast `error.message`,
  // and a fixed "Unable to open …" told the person nothing about why.
  if (createResponse.error) throw createResponse.error;
  if (!chat?.id) {
    throw new Error(t("Unable to open agent DM."));
  }
  return openChatTab({
    chatId: chat.id,
    title: params.title ?? chat.title ?? params.agentId,
  });
}

/**
 * Finds an existing private user↔user DM chat id from the local chat list.
 *
 * @param chats - Chats already loaded in the client store.
 * @param userId - Peer user id.
 * @param selfUserId - Current user id (both must be participants).
 * @returns Matching chat id when present.
 */
export function findUserDmChatId(
  chats: Array<Pick<Chat, "id" | "kind" | "participants">>,
  userId: string,
  selfUserId: string,
): string | undefined {
  const match = chats.find((chat) => {
    if (chat.kind && chat.kind !== "dm") {
      return false;
    }
    const participants = chat.participants ?? [];
    const hasPeer = participants.some(
      (participant) =>
        participant.type === "user" && participant.id === userId,
    );
    const hasSelf = participants.some(
      (participant) =>
        participant.type === "user" && participant.id === selfUserId,
    );
    const hasAgent = participants.some(
      (participant) => participant.type === "agent",
    );
    return hasPeer && hasSelf && !hasAgent;
  });
  return match?.id;
}

/**
 * The people the Team tab offers a conversation with: everyone in the
 * workspace directory but the person looking at it.
 *
 * The directory lists every account, the viewer included — the task assignee
 * pickers read the same list and need to offer "me". Offering the viewer a DM
 * with themselves opened a conversation nobody could delete, whose messages
 * the orchestrator answered.
 *
 * @param users - Workspace directory users.
 * @param selfUserId - The signed-in user, when known.
 * @returns The users other than the viewer.
 */
export function teamPeople<T extends { id: string }>(
  users: T[],
  selfUserId: string | undefined,
): T[] {
  return selfUserId ? users.filter((user) => user.id !== selfUserId) : users;
}

/**
 * Finds or creates a private user↔user DM, then opens it as a viewport tab.
 *
 * Same fix as `openAgentDmTab` just above, for the same reason: there is no
 * `chat.findOrCreateDm` command, so this always threw. The caller
 * (`chat-team-list.tsx`) already does its own `findUserDmChatId` check
 * before falling back to this — this only needs to build the create call
 * correctly, with both participants (the peer and this workspace's own
 * signed-in user, read from the auth store since callers only ever had a
 * peer id to give this).
 *
 * @param params - Peer user id and optional title.
 * @returns The focused or newly created tab id.
 */
export async function openUserDmTab(params: {
  userId: string;
  title?: string;
}): Promise<string> {
  const listResponse = await aos.client.chat.list.query({
    query: { kind: "dm" },
  });
  // A list that could not be read is not an empty list: treating it as one
  // went on to create a second DM with someone who already had one.
  if (listResponse.error) throw listResponse.error;
  const chats = (listResponse?.data?.chats ?? []) as Chat[];
  const selfUserId = aos.stores.auth.state.user?.id;
  const existingId = selfUserId
    ? findUserDmChatId(chats, params.userId, selfUserId)
    : undefined;
  if (existingId) {
    const existing = chats.find((chat) => chat.id === existingId);
    return openChatTab({
      chatId: existingId,
      title: params.title ?? existing?.title ?? params.userId,
    });
  }

  const participants: Array<{ type: "user"; id: string }> = [
    { type: "user", id: params.userId },
  ];
  // Not twice: a peer who is the signed-in person is already named above, and
  // naming them again stored the same user as both participants.
  if (selfUserId && selfUserId !== params.userId) {
    participants.push({ type: "user", id: selfUserId });
  }
  const createResponse = await aos.client.chat.create.mutate({
    body: {
      title: params.title ?? params.userId,
      kind: "dm",
      participants,
    },
  });
  const chat = (
    createResponse as { data?: { chat?: Chat }; error?: unknown } | null | undefined
  )?.data?.chat;
  // The refusal itself, when there is one: callers toast `error.message`,
  // and a fixed "Unable to open …" told the person nothing about why.
  if (createResponse.error) throw createResponse.error;
  if (!chat?.id) {
    throw new Error(t("Unable to open user DM."));
  }
  return openChatTab({
    chatId: chat.id,
    title: params.title ?? chat.title ?? params.userId,
  });
}

/**
 * Names a chat's tab after the conversation, once the conversation is known.
 *
 * A tab opened from a deep link (`/chats/<id>`) or restored from a previous
 * session has only the id to go on, and nothing renamed it later: the tab
 * read "42ea4b04-1569-45cc-aa6c…" beside a header reading "Luara", for good.
 *
 * @param chatId - Chat whose tab to rename.
 * @param title - What the conversation is called.
 */
export function syncChatTabTitle(chatId: string, title: string): void {
  const next = title.trim();
  if (!next) return;
  const tab = aos.stores.viewport.state.tabs.items.find(
    (item) => item.type === "chat" && getTabChatId(item) === chatId,
  );
  if (tab && tab.title !== next) {
    aos.stores.viewport.actions.updateTab(tab.id, { title: next });
  }
}

/**
 * Closes the viewport tab bound to a chat id, if one is open.
 *
 * @param chatId - Chat id to close.
 */
export function closeChatTab(chatId: string): void {
  const tab = aos.stores.viewport.state.tabs.items.find(
    (item) => item.type === "chat" && getTabChatId(item) === chatId,
  );

  if (!tab) {
    return;
  }

  aos.stores.viewport.actions.closeTab(tab.id);
}
