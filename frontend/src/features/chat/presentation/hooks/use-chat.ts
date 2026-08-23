import * as React from "react";
import { aos } from "@/app/aos";
import { useRealtime } from "@/hooks/use-realtime";
import type { Chat, ChatMessage } from "@/features/chat/interfaces/chat.interfaces";

interface UseChatParams {
  chatId: string;
  enabled?: boolean;
}

export interface UseChatResult {
  chat: Chat | null;
  messages: ChatMessage[];
  persistedMessageIds: string[];
  isLoading: boolean;
  isRefreshing: boolean;
  refresh: () => void;
  /** Inserts (or upserts by id) a message straight into local state — the optimistic echo for a just-submitted send. */
  appendMessage: (message: ChatMessage) => void;
  /** Swaps a locally-echoed message for the server-persisted one once the send mutation resolves. */
  replaceMessage: (previousId: string, message: ChatMessage) => void;
  /** Drops a locally-echoed message that failed to send. */
  removeMessage: (id?: string) => void;
}

function getMessageUpdatedAt(message: ChatMessage): number {
  const updatedAt = message.metadata?.updatedAt ?? message.metadata?.createdAt;
  const timestamp = updatedAt ? new Date(updatedAt).getTime() : 0;

  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function isCompletedAgentMessage(message: ChatMessage): boolean {
  return (
    message.metadata?.type === "agent" &&
    message.metadata.execution?.status === "completed"
  );
}

function shouldReplaceMessage(
  current: ChatMessage,
  incoming: ChatMessage,
): boolean {
  if (isCompletedAgentMessage(incoming)) {
    return true;
  }

  if (isCompletedAgentMessage(current)) {
    return false;
  }

  if ((incoming.parts?.length ?? 0) > (current.parts?.length ?? 0)) {
    return true;
  }

  if ((incoming.parts?.length ?? 0) < (current.parts?.length ?? 0)) {
    return false;
  }

  return getMessageUpdatedAt(incoming) >= getMessageUpdatedAt(current);
}

function upsertMessage(
  messages: ChatMessage[],
  incoming: ChatMessage,
): ChatMessage[] {
  const index = messages.findIndex((message) => message.id === incoming.id);

  if (index === -1) {
    return [...messages, incoming];
  }

  const current = messages[index];
  if (!shouldReplaceMessage(current, incoming)) {
    return messages;
  }

  const nextMessages = [...messages];
  nextMessages[index] = incoming;
  return nextMessages;
}

function mergeMessages(
  currentMessages: ChatMessage[],
  incomingMessages: ChatMessage[],
): ChatMessage[] {
  return incomingMessages.reduce(upsertMessage, currentMessages);
}

/**
 * Keeps a chat synchronized with its persisted snapshot and realtime message events.
 *
 * @description Loads the initial chat through the typed API and applies websocket
 * refresh payloads locally when they include a message snapshot. Payloads without
 * a message still trigger a full refetch for explicit reconciliation.
 * @param params - Chat synchronization parameters.
 * @returns Live chat state, message list, persisted message ids, loading flags, and manual refresh action.
 *
 * @example
 * const chat = useChat({ chatId })
 * if (!chat.chat) return null
 * return <ChatMessageList chat={chat.chat} persistedMessageIds={chat.persistedMessageIds} />
 */
export function useChat({ chatId, enabled = true }: UseChatParams): UseChatResult {
  const [chat, setChat] = React.useState<Chat | null>(null);

  /**
   * Set when a `chat:refresh` payload carries `replace: true` (context clear,
   * bulk message overwrite). The next query snapshot is then applied verbatim
   * instead of being merged with local streaming state, so removed messages
   * never stay visible.
   */
  const replaceMessagesRef = React.useRef(false);

  const chatQuery = aos.client.chat.getById.useQuery({
    params: { chat: chatId },
    enabled,
    onSuccess: (data) => {
      const nextChat = data?.chat;

      if (!nextChat) {
        return;
      }

      setChat((currentChat) => {
        if (!currentChat || currentChat.id !== nextChat.id) {
          return nextChat;
        }

        if (replaceMessagesRef.current) {
          replaceMessagesRef.current = false;
          return nextChat;
        }

        return {
          ...nextChat,
          messages: mergeMessages(
            (currentChat.messages ?? []) as ChatMessage[],
            (nextChat.messages ?? []) as ChatMessage[],
          ),
        };
      });
    },
  });

  const applyMessageSnapshot = React.useCallback(
    (message: ChatMessage) => {
      setChat((currentChat) => {
        if (!currentChat) {
          return currentChat;
        }

        return {
          ...currentChat,
          messages: upsertMessage(
            (currentChat.messages ?? []) as ChatMessage[],
            message,
          ),
        };
      });
    },
    [],
  );

  const replaceMessage = React.useCallback(
    (previousId: string, message: ChatMessage) => {
      setChat((currentChat) => {
        if (!currentChat) {
          return currentChat;
        }

        const withoutPrevious = (
          (currentChat.messages ?? []) as ChatMessage[]
        ).filter((existing) => existing.id !== previousId);

        return {
          ...currentChat,
          messages: upsertMessage(withoutPrevious, message),
        };
      });
    },
    [],
  );

  const removeMessage = React.useCallback((id?: string) => {
    if (!id) {
      return;
    }

    setChat((currentChat) => {
      if (!currentChat) {
        return currentChat;
      }

      return {
        ...currentChat,
        messages: ((currentChat.messages ?? []) as ChatMessage[]).filter(
          (existing) => existing.id !== id,
        ),
      };
    });
  }, []);

  useRealtime(
    "chat:refresh",
    (payload) => {
      if (payload.chatId !== chatId) {
        return;
      }

      if (payload.message) {
        applyMessageSnapshot(payload.message);
        return;
      }

      if (payload.replace) {
        replaceMessagesRef.current = true;
      }

      chatQuery.refetch();
    },
    [chatId, applyMessageSnapshot, chatQuery.refetch],
    enabled,
  );

  const messages = React.useMemo(
    () => (chat?.messages ?? []) as ChatMessage[],
    [chat?.messages],
  );
  const persistedMessageIds = React.useMemo(() => {
    const ids = new Set(
      ((chatQuery.data?.chat?.messages ?? []) as ChatMessage[]).map(
        (message) => message.id,
      ),
    );

    for (const message of messages) {
      if (isCompletedAgentMessage(message)) {
        ids.add(message.id);
      }
    }

    return [...ids];
  }, [chatQuery.data?.chat?.messages, messages]);

  return {
    chat,
    messages,
    persistedMessageIds,
    isLoading: chatQuery.isLoading,
    isRefreshing: chatQuery.isFetching,
    refresh: chatQuery.refetch,
    appendMessage: applyMessageSnapshot,
    replaceMessage,
    removeMessage,
  };
}
