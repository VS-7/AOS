import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import {
  ChatKindHelper,
  type ChatKind,
} from "@/features/chat/services/chat/chat-kind.helper";

export type ChatSearchHit = {
  chatId: string;
  title: string;
  kind: ChatKind;
  updatedAt?: string | Date;
  subtitle?: string;
  score: number;
};

/**
 * Scores and ranks chats for the sidebar finder.
 *
 * Empty query → recent chats (by updatedAt). Non-empty → title/id/task/routine
 * match with recency boost.
 */
export class ChatSearchHelper {
  /**
   * Builds ranked search hits across all chat kinds + agent DM stubs.
   */
  public static search(input: {
    query: string;
    chats: Chat[];
    agents: Agent[];
    agentIds: ReadonlySet<string>;
    limit?: number;
  }): ChatSearchHit[] {
    const limit = input.limit ?? 24;
    const q = input.query.trim().toLowerCase();
    const hits: ChatSearchHit[] = [];

    for (const chat of input.chats) {
      const kind = ChatKindHelper.classify(chat, input.agentIds);
      const title = chat.title || chat.id;
      const haystack = [
        title,
        chat.id,
        chat.task ?? "",
        chat.routine ?? "",
        kind,
      ]
        .join(" ")
        .toLowerCase();

      const score = q
        ? ChatSearchHelper._score(haystack, title.toLowerCase(), q)
        : ChatSearchHelper._recency_score(chat.updatedAt);

      if (q && score <= 0) {
        continue;
      }

      hits.push({
        chatId: chat.id,
        title,
        kind,
        updatedAt: chat.updatedAt,
        subtitle:
          kind === "task"
            ? chat.task
            : kind === "run"
              ? chat.routine
              : kind === "dm"
                ? "DM"
                : undefined,
        score,
      });
    }

    // Agent roster entries that don't have a chat file yet still appear.
    if (q) {
      for (const agent of input.agents) {
        const existingDmId = input.chats.find(
          (chat) =>
            chat.id === agent.id ||
            (chat.kind === "dm" &&
              (chat.participants ?? []).some(
                (participant) =>
                  participant.type === "agent" && participant.id === agent.id,
              )),
        )?.id;

        if (existingDmId && hits.some((hit) => hit.chatId === existingDmId)) {
          continue;
        }

        const title = agent.name || agent.id;
        const haystack = `${title} ${agent.id} dm agent`.toLowerCase();
        const score = ChatSearchHelper._score(
          haystack,
          title.toLowerCase(),
          q,
        );

        if (score <= 0) {
          continue;
        }

        hits.push({
          chatId: existingDmId ?? agent.id,
          title,
          kind: "dm",
          subtitle: "DM",
          score,
        });
      }
    }

    return hits
      .sort((left, right) => {
        if (right.score !== left.score) {
          return right.score - left.score;
        }

        return (
          ChatSearchHelper._to_time(right.updatedAt) -
          ChatSearchHelper._to_time(left.updatedAt)
        );
      })
      .slice(0, limit);
  }

  private static _score(haystack: string, title: string, query: string): number {
    if (title === query) return 100;
    if (title.startsWith(query)) return 80;
    if (title.includes(query)) return 60;
    if (haystack.includes(query)) return 40;

    const tokens = query.split(/\s+/).filter(Boolean);
    if (tokens.length > 1 && tokens.every((token) => haystack.includes(token))) {
      return 30;
    }

    return 0;
  }

  private static _recency_score(updatedAt?: string | Date): number {
    const ageMs = Date.now() - ChatSearchHelper._to_time(updatedAt);
    return Math.max(1, 50 - Math.floor(ageMs / (1000 * 60 * 60 * 6)));
  }

  private static _to_time(value?: string | Date): number {
    if (!value) return 0;
    const ms = value instanceof Date ? value.getTime() : Date.parse(value);
    return Number.isNaN(ms) ? 0 : ms;
  }
}
