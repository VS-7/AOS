import type { UIMessage } from "ai"
import type { Agent } from "@/features/agent/interfaces/agent.interfaces"
import type { Chat, ChatMessageMetadata } from "@/features/chat/interfaces/chat.interfaces"
import type { WorkspaceDirectoryUser } from "@/features/workspace/interfaces/directory.interfaces"
import { MessageHelper } from "./message.helper"

interface ResolveParticipantOptions {
  agents: Agent[]
  chat: Chat
  message: UIMessage<ChatMessageMetadata>
  /** Viewer display name — fallback when the speaker is the current user. */
  userName: string
  /** Viewer user id — marks self vs peer in multi-user threads. */
  selfUserId?: string
  /** Workspace directory users keyed by id (from `stores.workspace.directory`). */
  usersById?: ReadonlyMap<string, WorkspaceDirectoryUser>
}

export interface ChatMessageParticipant {
  id: string
  kind: "user" | "agent"
  label: string
  image?: string
}

export class ChatThreadHelper {
  public static getMessageTextParts(message: UIMessage<ChatMessageMetadata>) {
    return MessageHelper.getMessageTextParts(message).map((text) => text.trim())
  }

  public static getMessageText(message: UIMessage<ChatMessageMetadata>) {
    return MessageHelper.getMessageText(message).trim()
  }

  public static getMessageTimestamp(message: UIMessage<ChatMessageMetadata>) {
    const metadata = message.metadata
    const createdAt = metadata?.createdAt

    if (!createdAt) {
      return null
    }

    // @ts-expect-error - Expected don`t remove it!
    const date = createdAt instanceof Date ? createdAt : new Date(createdAt)

    if (Number.isNaN(date.getTime())) {
      return null
    }

    return date
  }

  public static resolveParticipant({
    agents,
    chat,
    message,
    userName,
    selfUserId,
    usersById,
  }: ResolveParticipantOptions): ChatMessageParticipant {
    const metadata = message.metadata as ChatMessageMetadata | undefined

    if (metadata?.type === "agent") {
      const agent = agents.find((item) => item.id === metadata.data.id)

      return {
        id: metadata.data.id,
        kind: "agent",
        label: agent?.name || chat.title,
        image: agent?.image,
      }
    }

    if (metadata?.type === "user") {
      return ChatThreadHelper._resolve_user_participant({
        userId: metadata.data.id,
        userName,
        selfUserId,
        usersById,
      })
    }

    if (message.role === "assistant") {
      const directAgent = agents.find((item) => item.id === chat.id)

      return {
        id: directAgent?.id || chat.id,
        kind: "agent",
        label: directAgent?.name || chat.title,
        image: directAgent?.image,
      }
    }

    return ChatThreadHelper._resolve_user_participant({
      userId: selfUserId ?? "user",
      userName,
      selfUserId,
      usersById,
    })
  }

  /**
   * Resolves a human speaker label/image from the workspace directory.
   */
  private static _resolve_user_participant(params: {
    userId: string
    userName: string
    selfUserId?: string
    usersById?: ReadonlyMap<string, WorkspaceDirectoryUser>
  }): ChatMessageParticipant {
    const profile = params.usersById?.get(params.userId)
    const isSelf =
      Boolean(params.selfUserId) && params.userId === params.selfUserId

    const label =
      profile?.name?.trim() ||
      profile?.username?.trim() ||
      (isSelf ? params.userName || "You" : undefined) ||
      "Teammate"

    return {
      id: params.userId,
      kind: "user",
      label,
      image: profile?.image,
    }
  }

  public static getInitials(value: string) {
    return value
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part.slice(0, 1).toUpperCase())
      .join("")
  }

  public static formatMessageTime(value: Date) {
    return new Intl.DateTimeFormat("en-US", {
      hour: "numeric",
      minute: "2-digit",
    }).format(value)
  }

  public static formatMessageDay(value: Date) {
    return new Intl.DateTimeFormat("en-US", {
      weekday: "short",
      month: "short",
      day: "numeric",
    }).format(value)
  }

  public static isSameDay(left: Date | null, right: Date | null) {
    if (!left || !right) {
      return false
    }

    return (
      left.getFullYear() === right.getFullYear() &&
      left.getMonth() === right.getMonth() &&
      left.getDate() === right.getDate()
    )
  }

  /**
   * Maximum gap between consecutive messages that still collapse into one visual group.
   */
  public static readonly MESSAGE_GROUP_WINDOW_MS = 5 * 60 * 1000

  /**
   * Whether two consecutive messages should share avatar/header chrome.
   *
   * Same participant within {@link ChatThreadHelper.MESSAGE_GROUP_WINDOW_MS}.
   * When either timestamp is missing (optimistic/local messages), participant
   * match alone is enough so rapid sends still group.
   */
  public static isGroupedWithNeighbor(options: {
    participantId: string
    neighborParticipantId?: string
    timestamp: Date | null
    neighborTimestamp: Date | null
  }): boolean {
    const {
      participantId,
      neighborParticipantId,
      timestamp,
      neighborTimestamp,
    } = options

    if (!neighborParticipantId || neighborParticipantId !== participantId) {
      return false
    }

    if (!timestamp || !neighborTimestamp) {
      return true
    }

    return (
      Math.abs(timestamp.getTime() - neighborTimestamp.getTime()) <
      ChatThreadHelper.MESSAGE_GROUP_WINDOW_MS
    )
  }
}
