import type { Chat } from "../../interfaces/chat.interfaces";
// Re-exported (not just imported): three sidebar files (`chat-row.tsx`,
// `chat-row-kind-icon.tsx`, `chat-search.helper.ts`) import `FractalChatKind`
// from this module, not `chat.interfaces.ts` directly.
export type { FractalChatKind } from "../../interfaces/chat.interfaces";
import type { FractalChatKind } from "../../interfaces/chat.interfaces";

/**
 * Occupancy map from {@link aos.stores.agent} — chatId → agent ids processing.
 */
export type FractalChatOccupancy = Record<string, string[]>;

/**
 * Pure classification, search, and live-sort helpers for chat list surfaces.
 *
 * Shared by {@link FractalChatService.list} and the workspace Chat sidebar so
 * kind detection stays consistent across HTTP, CLI, and FE.
 */
export class FractalChatKindHelper {
  /**
   * Maximum idle (non-processing) chats shown on the Live tab after active ones.
   */
  public static readonly LIVE_IDLE_CAP = 20;

  /**
   * Classifies a chat into a surface kind.
   *
   * Priority: persisted `kind` → DM participants / legacy agent id → task →
   * routine → external → channel.
   *
   * @param chat - Chat entity (may omit messages).
   * @param agentIds - Set of registered agent slugs/ids.
   * @returns Kind used by sidebar tabs and list filters.
   *
   * @example
   * ```typescript
   * FractalChatKindHelper.classify(chat, new Set(["atlas"])); // "dm" | "channel" | …
   * ```
   */
  public static classify(
    chat: Pick<
      Chat,
      "id" | "task" | "routine" | "metadata" | "kind" | "participants"
    >,
    agentIds: ReadonlySet<string>,
  ): FractalChatKind {
    const persisted = chat.kind as string | undefined;

    // [Condition]: Legacy pre-multi-user label — agent DMs were kind "agent".
    if (persisted === "agent") {
      return "dm";
    }

    // [Condition]: Explicit non-default kinds are authoritative.
    // Schema defaults missing kind to "channel", so "channel" alone must not
    // short-circuit task/run/dm heuristics (that broke sidebar tab grouping).
    if (
      persisted === "dm" ||
      persisted === "task" ||
      persisted === "run" ||
      persisted === "external"
    ) {
      return persisted;
    }

    // [Condition]: Private DM with an agent participant.
    const hasAgentParticipant = (chat.participants ?? []).some(
      (participant) =>
        participant.type === "agent" && agentIds.has(participant.id),
    );
    if (hasAgentParticipant) {
      return "dm";
    }

    // [Condition]: Legacy agent DMs used the agent id as the chat id.
    if (agentIds.has(chat.id)) {
      return "dm";
    }

    // [Condition]: User↔user private DMs without agent participants.
    const userParticipants = (chat.participants ?? []).filter(
      (participant) => participant.type === "user",
    );
    if (userParticipants.length >= 2 && !hasAgentParticipant) {
      return "dm";
    }

    // [Condition]: Task-linked execution chats.
    if (chat.task) {
      return "task";
    }

    // [Condition]: Routine run chats.
    if (chat.routine) {
      return "run";
    }

    // [Condition]: External messenger bindings are not workspace channels.
    if (chat.metadata?.channel) {
      return "external";
    }

    // [Return]: Free workspace channel (also when persisted kind is "channel").
    return "channel";
  }

  /**
   * Returns whether a chat matches a free-text query (title, id, task, routine, kind).
   *
   * @param chat - Chat list item.
   * @param query - Raw search string (case-insensitive substring).
   * @param agentIds - Agent ids used when deriving kind for the haystack.
   * @returns True when any haystack field contains the query.
   */
  public static matches_query(
    chat: Pick<
      Chat,
      | "id"
      | "title"
      | "task"
      | "routine"
      | "metadata"
      | "kind"
      | "participants"
    >,
    query: string,
    agentIds: ReadonlySet<string>,
  ): boolean {
    const term = query.trim().toLowerCase();
    if (!term) {
      return true;
    }

    const kind = FractalChatKindHelper.classify(chat, agentIds);
    const haystack = [chat.id, chat.title, chat.task, chat.routine, kind]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();

    return haystack.includes(term);
  }

  /**
   * Counts chats per surface kind.
   *
   * @param chats - Classified chat list items.
   * @returns Count map for every kind key.
   */
  public static count_groups(
    chats: Array<{ kind: FractalChatKind }>,
  ): Record<FractalChatKind, number> {
    const groups: Record<FractalChatKind, number> = {
      dm: 0,
      task: 0,
      run: 0,
      external: 0,
      channel: 0,
    };

    for (const chat of chats) {
      groups[chat.kind] += 1;
    }

    return groups;
  }

  /**
   * Returns whether any agent is currently processing in the given chat.
   *
   * @param chatId - Chat identifier.
   * @param occupancy - Agent occupancy map keyed by chat id.
   * @returns True when at least one agent id is listed for the chat.
   */
  public static isProcessing(
    chatId: string,
    occupancy: FractalChatOccupancy,
  ): boolean {
    const agents = occupancy[chatId];
    return Array.isArray(agents) && agents.length > 0;
  }

  /**
   * Filters chats to a single kind.
   *
   * @param chats - Full chat list.
   * @param kind - Target kind.
   * @param agentIds - Registered agent ids for DM detection.
   * @returns Chats matching the kind.
   */
  public static filterByKind(
    chats: Chat[],
    kind: FractalChatKind,
    agentIds: ReadonlySet<string>,
  ): Chat[] {
    return chats.filter(
      (chat) => FractalChatKindHelper.classify(chat, agentIds) === kind,
    );
  }

  /**
   * Counts processing chats for a kind (sidebar badge).
   *
   * @param chats - Chat list.
   * @param kind - Target kind.
   * @param occupancy - Live occupancy map.
   * @param agentIds - Registered agent ids.
   * @returns Number of processing chats in that kind.
   */
  public static countProcessingByKind(
    chats: Chat[],
    kind: FractalChatKind,
    occupancy: FractalChatOccupancy,
    agentIds: ReadonlySet<string>,
  ): number {
    return FractalChatKindHelper.filterByKind(chats, kind, agentIds).filter(
      (chat) => FractalChatKindHelper.isProcessing(chat.id, occupancy),
    ).length;
  }

  /**
   * Returns chats that currently have agent occupancy, newest first.
   *
   * @param chats - Candidate chats.
   * @param occupancy - Live occupancy map.
   * @returns Processing chats sorted by updatedAt descending.
   */
  public static listProcessing(
    chats: Chat[],
    occupancy: FractalChatOccupancy,
  ): Chat[] {
    return chats
      .filter((chat) =>
        FractalChatKindHelper.isProcessing(chat.id, occupancy),
      )
      .sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)));
  }

  /**
   * Sorts Live-tab chats: processing first, then recent idle capped.
   *
   * @param chats - Candidate chats.
   * @param occupancy - Live occupancy map.
   * @param idleCap - Optional idle cap (defaults to {@link LIVE_IDLE_CAP}).
   * @returns Sorted + capped list for the Live tab.
   */
  public static sortLive(
    chats: Chat[],
    occupancy: FractalChatOccupancy,
    idleCap: number = FractalChatKindHelper.LIVE_IDLE_CAP,
  ): Chat[] {
    const processing: Chat[] = [];
    const idle: Chat[] = [];

    for (const chat of chats) {
      if (FractalChatKindHelper.isProcessing(chat.id, occupancy)) {
        processing.push(chat);
      } else {
        idle.push(chat);
      }
    }

    const byUpdatedDesc = (a: Chat, b: Chat) =>
      String(b.updatedAt).localeCompare(String(a.updatedAt));

    processing.sort(byUpdatedDesc);
    idle.sort(byUpdatedDesc);

    return [...processing, ...idle.slice(0, idleCap)];
  }

  /**
   * Returns whether a DM is a user↔agent thread (vs user↔user).
   *
   * @param chat - Chat with participants.
   * @param agentIds - Registered agent ids.
   * @returns True when an agent participant is present (or legacy slug id).
   */
  public static is_agent_dm(
    chat: Pick<Chat, "id" | "kind" | "participants">,
    agentIds: ReadonlySet<string>,
  ): boolean {
    if (agentIds.has(chat.id)) {
      return true;
    }
    return (chat.participants ?? []).some(
      (participant) =>
        participant.type === "agent" && agentIds.has(participant.id),
    );
  }

  /**
   * Collects agent ids with pending/running work inside a single chat.
   *
   * @param chat - Chat with messages (runs / execution metadata).
   * @returns Unique agent ids currently processing the chat.
   */
  public static active_agent_ids(
    chat: Pick<Chat, "messages">,
  ): string[] {
    const agents = new Set<string>();

    for (const message of chat.messages ?? []) {
      for (const run of message.metadata?.runs ?? []) {
        if (run.status === "pending" || run.status === "running") {
          agents.add(run.agentId);
        }
      }

      if (
        message.metadata?.type === "agent" &&
        message.metadata.execution &&
        (message.metadata.execution.status === "pending" ||
          message.metadata.execution.status === "running")
      ) {
        agents.add(message.metadata.execution.agentId);
      }
    }

    return [...agents];
  }

  /**
   * Builds agent → processing-chat index from chat records (server seed).
   *
   * @param chats - Chats that include messages for run inspection.
   * @param agentIds - Registered agent ids for kind classification.
   * @returns Map of agent id → processing chat summaries.
   */
  public static index_processing_by_agent(
    chats: Chat[],
    agentIds: ReadonlySet<string>,
  ): Map<
    string,
    Array<{ chatId: string; title: string; kind: FractalChatKind }>
  > {
    const index = new Map<
      string,
      Array<{ chatId: string; title: string; kind: FractalChatKind }>
    >();

    for (const chat of chats) {
      const active = FractalChatKindHelper.active_agent_ids(chat);
      if (active.length === 0) {
        continue;
      }

      const kind = FractalChatKindHelper.classify(chat, agentIds);
      const entry = {
        chatId: chat.id,
        title: chat.title,
        kind,
      };

      for (const agentId of active) {
        const list = index.get(agentId) ?? [];
        list.push(entry);
        index.set(agentId, list);
      }
    }

    return index;
  }

  /**
   * Lists processing chats for one agent from the live occupancy map (client).
   *
   * @param agentId - Agent slug.
   * @param occupancy - Live chatId → agentIds map.
   * @param chats - Chat list (messages optional — title/kind only).
   * @param agentIds - Registered agent ids for kind classification.
   * @returns Processing chat summaries for the agent.
   */
  public static list_processing_for_agent(
    agentId: string,
    occupancy: FractalChatOccupancy,
    chats: Array<
      Pick<
        Chat,
        | "id"
        | "title"
        | "kind"
        | "task"
        | "routine"
        | "metadata"
        | "participants"
      >
    >,
    agentIds: ReadonlySet<string>,
  ): Array<{ chatId: string; title: string; kind: FractalChatKind }> {
    const byId = new Map(chats.map((chat) => [chat.id, chat]));
    const result: Array<{
      chatId: string;
      title: string;
      kind: FractalChatKind;
    }> = [];

    for (const [chatId, agents] of Object.entries(occupancy)) {
      if (!agents.includes(agentId)) {
        continue;
      }

      const chat = byId.get(chatId);
      if (!chat) {
        result.push({ chatId, title: chatId, kind: "channel" });
        continue;
      }

      result.push({
        chatId,
        title: chat.title,
        kind: FractalChatKindHelper.classify(chat, agentIds),
      });
    }

    return result;
  }

  /**
   * Inverts occupancy (chat → agents) into the agent-store shape seed.
   *
   * @param processingByAgent - Agent → processing chats.
   * @returns Occupancy map chatId → agentIds.
   */
  public static occupancy_from_processing_index(
    processingByAgent: Map<
      string,
      Array<{ chatId: string; title: string; kind: FractalChatKind }>
    >,
  ): FractalChatOccupancy {
    const occupancy: FractalChatOccupancy = {};

    for (const [agentId, entries] of processingByAgent) {
      for (const entry of entries) {
        const current = occupancy[entry.chatId] ?? [];
        if (!current.includes(agentId)) {
          occupancy[entry.chatId] = [...current, agentId];
        }
      }
    }

    return occupancy;
  }
}
