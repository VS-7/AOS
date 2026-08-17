import type { Chat, Message } from "@/features/chat/interfaces/chat.interfaces";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";

interface ResolveParticipantOptions {
  agents: Agent[];
  chat: Chat;
  message: Message;
  /** Viewer display name — fallback when the speaker is the current user. */
  userName: string;
  /** Viewer identifier — marks self vs peer. AOS has no user directory yet
   * (single-operator model), so this is almost always the literal "user". */
  selfUserId?: string;
}

export interface ChatMessageParticipant {
  id: string;
  kind: "user" | "agent";
  label: string;
  image?: string;
}

/**
 * Ported from the original's ChatThreadHelper, adapted to AOS's Message shape
 * (Author/Runs are top-level Go struct fields, not AI-SDK metadata) and to
 * AOS having no workspace directory of human users yet — every non-agent
 * speaker resolves to the viewer.
 */
export class ChatThreadHelper {
  public static getMessageTimestamp(message: Message): Date | null {
    if (!message.createdAt) return null;
    const date = new Date(message.createdAt);
    return Number.isNaN(date.getTime()) ? null : date;
  }

  public static resolveParticipant({
    agents,
    chat,
    message,
    userName,
    selfUserId,
  }: ResolveParticipantOptions): ChatMessageParticipant {
    if (message.author?.type === "agent") {
      const agent = agents.find((item) => item.id === message.author?.id);
      return {
        id: message.author.id,
        kind: "agent",
        label: agent?.name || chat.title,
        image: agent?.image,
      };
    }

    if (message.role === "assistant") {
      const directAgent = agents.find((item) => item.id === chat.id);
      return {
        id: directAgent?.id || chat.id,
        kind: "agent",
        label: directAgent?.name || chat.title,
        image: directAgent?.image,
      };
    }

    // Every human speaker is the viewer until AOS has a workspace directory
    // of users — see docs/06 - Frontend/React 19 e Bindings.md.
    return {
      id: message.author?.id ?? selfUserId ?? "user",
      kind: "user",
      label: userName || "You",
    };
  }

  public static getInitials(value: string): string {
    return value
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join("");
  }

  public static formatMessageTime(value: Date): string {
    return new Intl.DateTimeFormat("en-US", {
      hour: "numeric",
      minute: "2-digit",
    }).format(value);
  }

  public static formatMessageDay(value: Date): string {
    return new Intl.DateTimeFormat("en-US", {
      weekday: "short",
      month: "short",
      day: "numeric",
    }).format(value);
  }

  public static isSameDay(left: Date | null, right: Date | null): boolean {
    if (!left || !right) return false;
    return (
      left.getFullYear() === right.getFullYear() &&
      left.getMonth() === right.getMonth() &&
      left.getDate() === right.getDate()
    );
  }

  /** Maximum gap between consecutive messages that still collapse into one visual group. */
  public static readonly MESSAGE_GROUP_WINDOW_MS = 5 * 60 * 1000;

  /** Whether two consecutive messages should share avatar/header chrome. */
  public static isGroupedWithNeighbor(options: {
    participantId: string;
    neighborParticipantId?: string;
    timestamp: Date | null;
    neighborTimestamp: Date | null;
  }): boolean {
    const { participantId, neighborParticipantId, timestamp, neighborTimestamp } = options;

    if (!neighborParticipantId || neighborParticipantId !== participantId) return false;
    if (!timestamp || !neighborTimestamp) return true;

    return (
      Math.abs(timestamp.getTime() - neighborTimestamp.getTime()) <
      ChatThreadHelper.MESSAGE_GROUP_WINDOW_MS
    );
  }
}
