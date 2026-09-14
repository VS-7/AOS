import type { UIMessage } from "ai"
import type { Agent } from "@/features/agent/interfaces/agent.interfaces"
import type { Chat, ChatMessageMetadata } from "@/features/chat/interfaces/chat.interfaces"
import type { WorkspaceDirectoryUser } from "@/features/workspace/interfaces/directory.interfaces"
import { MessageHelper } from "./message.helper"
import { getLocale, t } from "@/lib/i18n"

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
      (isSelf ? params.userName || t("You") : undefined) ||
      t("Teammate")

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

  // In the interface's language, not a fixed en-US: a Portuguese window read
  // "Sat, Sep 12" over "5:49 PM".
  public static formatMessageTime(value: Date, locale: string = getLocale()) {
    return new Intl.DateTimeFormat(locale, {
      hour: "numeric",
      minute: "2-digit",
    }).format(value)
  }

  public static formatMessageDay(value: Date, locale: string = getLocale()) {
    return new Intl.DateTimeFormat(locale, {
      weekday: "short",
      month: "short",
      day: "numeric",
    }).format(value)
  }

  /**
   * "You and Ana reacted with 👍".
   *
   * The daemon records who reacted by id — a user's id, an agent's slug —
   * never by name. The tooltip used to compare those ids with the viewer's
   * display name, so nothing ever matched and it printed the raw UUID.
   */
  public static formatReactionTooltip(options: {
    actors: string[]
    emoji: string
    selfUserId?: string
    usersById?: ReadonlyMap<string, Pick<WorkspaceDirectoryUser, "name" | "username">>
    agents?: Array<Pick<Agent, "id" | "name">>
  }): string {
    const nameOf = (actor: string) => {
      if (options.selfUserId && actor === options.selfUserId) return t("You")
      const user = options.usersById?.get(actor)
      const userName = user?.name?.trim() || user?.username?.trim()
      if (userName) return userName
      return options.agents?.find((agent) => agent.id === actor)?.name || actor
    }
    const names = options.actors.map(nameOf)
    const emoji = options.emoji

    if (names.length === 1) {
      return t("{{name}} reacted with {{emoji}}", { name: names[0], emoji })
    }
    if (names.length === 2) {
      return t("{{first}} and {{second}} reacted with {{emoji}}", { first: names[0], second: names[1], emoji })
    }
    const others = names.length - 2
    return others === 1
      ? t("{{first}}, {{second}} and 1 other reacted with {{emoji}}", { first: names[0], second: names[1], emoji })
      : t("{{first}}, {{second}} and {{count}} others reacted with {{emoji}}", {
          first: names[0],
          second: names[1],
          count: others,
          emoji,
        })
  }

  /**
   * Whether a newly appended message should pull the thread to the bottom.
   *
   * Always while the reader is already there. Otherwise only for the reader's
   * own message that the daemon has not confirmed yet — the echo of what they
   * just sent. Following only at the bottom meant a send from a scrolled-up
   * thread added the message below the fold and nothing visibly happened;
   * following everything would yank somebody reading history down whenever an
   * agent spoke.
   */
  public static shouldFollowNewest(options: {
    atBottom: boolean
    newest: UIMessage<ChatMessageMetadata> | undefined
    persistedIds: ReadonlySet<string>
    selfUserId?: string
  }): boolean {
    if (options.atBottom) return true
    const newest = options.newest
    if (!newest || newest.role !== "user" || options.persistedIds.has(newest.id)) {
      return false
    }
    const author = newest.metadata?.type === "user" ? newest.metadata.data?.id : undefined
    return Boolean(options.selfUserId) && author === options.selfUserId
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
