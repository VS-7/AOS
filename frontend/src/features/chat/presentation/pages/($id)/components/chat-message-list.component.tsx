import * as React from "react";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { isTextUIPart, isToolUIPart } from "ai";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { AgentToolThinkingHelper } from "@/features/agent/presentation/helpers/agent-tool-thinking.helper";
import type {
  Chat,
  ChatMessage,
  ChatReaction,
} from "@/features/chat/interfaces/chat.interfaces";
import type { WorkspaceDirectoryUser } from "@/features/workspace/interfaces/directory.interfaces";
import { ChatThreadHelper } from "@/features/chat/presentation/helpers/chat-thread.helper";
import {
  ChatMessageItem,
  type ChatMessageReaction,
} from "./chat-message-item.component";
import {
  Conversation,
  ConversationEmptyState,
} from "@/components/ui/conversation";
import {
  ChatTurnFailure,
  latestFailure,
  runsOf,
} from "@/features/chat/presentation/components/message/turn-failure";

interface ChatMessageListProps {
  agents: Agent[];
  chat: Chat;
  isRefreshing?: boolean;
  onReactionToggled?: () => void;
  persistedMessageIds?: string[];
  selfUserId?: string;
  userName: string;
  usersById?: ReadonlyMap<string, WorkspaceDirectoryUser>;
}

interface ChatMessageRowProps {
  agents: Agent[];
  chat: Chat;
  currentActor: string;
  isTogglingReaction: boolean;
  message: ChatMessage;
  nextMessage?: ChatMessage;
  onToggleReaction: (messageId: string, emoji: string) => void;
  persisted: boolean;
  previousMessage?: ChatMessage;
  selfUserId?: string;
  userName: string;
  usersById?: ReadonlyMap<string, WorkspaceDirectoryUser>;
}

function areChatsEquivalent(left: Chat, right: Chat) {
  return left.id === right.id && left.title === right.title;
}

function areRowsEqual(
  previous: ChatMessageRowProps,
  next: ChatMessageRowProps,
) {
  return (
    previous.agents === next.agents &&
    areChatsEquivalent(previous.chat, next.chat) &&
    previous.currentActor === next.currentActor &&
    previous.isTogglingReaction === next.isTogglingReaction &&
    previous.message === next.message &&
    previous.nextMessage === next.nextMessage &&
    previous.onToggleReaction === next.onToggleReaction &&
    previous.persisted === next.persisted &&
    previous.previousMessage === next.previousMessage &&
    previous.selfUserId === next.selfUserId &&
    previous.userName === next.userName &&
    previous.usersById === next.usersById
  );
}

function groupMessageReactions(
  reactions: ChatReaction[] | undefined,
  actor: string,
): ChatMessageReaction[] {
  const groupedReactions = new Map<string, ChatMessageReaction>();

  for (const reaction of reactions ?? []) {
    const current = groupedReactions.get(reaction.value);

    if (current) {
      current.count += 1;
      current.actors.push(reaction.actor);
      current.reacted = current.reacted || reaction.actor === actor;
      continue;
    }

    groupedReactions.set(reaction.value, {
      emoji: reaction.value,
      count: 1,
      actors: [reaction.actor],
      reacted: reaction.actor === actor,
    });
  }

  return [...groupedReactions.values()];
}

function canRenderMessage(message: ChatMessage) {
  const parts = message.parts ?? [];
  const textParts = parts
    .filter(isTextUIPart)
    .map((part) => part.text.trim())
    .filter(
      (text) => text.length > 0 && !text.startsWith("[system-reminder]:"),
    );

  return (
    textParts.length > 0 ||
    parts.some(isToolUIPart) ||
    parts.some((part) => part.type === "file") ||
    AgentToolThinkingHelper.toThinkingSteps(message).length > 0
  );
}

const ChatMessageRow = React.memo(function ChatMessageRow({
  agents,
  chat,
  currentActor,
  isTogglingReaction,
  message,
  nextMessage,
  onToggleReaction,
  persisted,
  previousMessage,
  selfUserId,
  userName,
  usersById,
}: ChatMessageRowProps) {
  const timestamp = ChatThreadHelper.getMessageTimestamp(message);
  const previousTimestamp = previousMessage
    ? ChatThreadHelper.getMessageTimestamp(previousMessage)
    : null;
  const showDayDivider = !ChatThreadHelper.isSameDay(
    timestamp,
    previousTimestamp,
  );
  const reactions = React.useMemo(
    () => groupMessageReactions(message.metadata?.reactions, currentActor),
    [currentActor, message.metadata?.reactions],
  );
  const reactionsDisabled = !persisted || isTogglingReaction;

  // A turn that failed is recorded on the message that asked for it, not as
  // an answer — see `turn-failure.tsx`. Rendering it here keeps the reason
  // attached to the message it belongs to.
  const failure = React.useMemo(
    () => latestFailure(runsOf(message)),
    [message],
  );

  const handleAddReaction = React.useCallback(
    (emoji: string) => onToggleReaction(message.id, emoji),
    [message.id, onToggleReaction],
  );
  const handleToggleReaction = React.useCallback(
    (emoji: string) => onToggleReaction(message.id, emoji),
    [message.id, onToggleReaction],
  );

  return (
    <React.Fragment>
      {showDayDivider && timestamp ? (
        <div className="sticky top-0 z-10 flex justify-center px-6 py-2 border-0!">
          <div className="rounded-full border bg-background/95 px-3 py-0.5 text-[11px] text-muted-foreground shadow-xs">
            {ChatThreadHelper.formatMessageDay(timestamp)}
          </div>
        </div>
      ) : null}

      <ChatMessageItem
        agents={agents}
        chat={chat}
        message={message}
        nextMessage={nextMessage}
        previousMessage={previousMessage}
        reactions={reactions}
        reactionsDisabled={reactionsDisabled}
        onAddReaction={handleAddReaction}
        onToggleReaction={handleToggleReaction}
        selfUserId={selfUserId}
        userName={userName}
        usersById={usersById}
        animateIn={!persisted}
      />

      {failure ? <ChatTurnFailure run={failure} /> : null}
    </React.Fragment>
  );
}, areRowsEqual);

export function ChatMessageList({
  agents,
  chat,
  onReactionToggled,
  persistedMessageIds = [],
  selfUserId,
  userName,
  usersById,
}: ChatMessageListProps) {
  const messages = React.useMemo(
    () =>
      ((chat.messages ?? []) as ChatMessage[]).filter(canRenderMessage),
    [chat.messages],
  );
  const persistedMessageIdSet = React.useMemo(
    () => new Set(persistedMessageIds),
    [persistedMessageIds],
  );
  const currentActor = selfUserId?.trim() || userName.trim() || "user";

  const { mutate: toggleReaction, loading: isTogglingReaction } =
    aos.client.chat.toggleReaction.useMutation({
      onSuccess: () => {
        onReactionToggled?.();
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message ||
            error?.message ||
            "Unable to update reaction.",
        );
      },
    });

  const updateMessageReaction = React.useCallback(
    (messageId: string, emoji: string) => {
      if (!persistedMessageIdSet.has(messageId) || isTogglingReaction) {
        return;
      }

      toggleReaction({
        params: {
          chat: chat.id,
        },
        body: {
          messageId,
          value: emoji,
          actor: currentActor,
        },
      });
    },
    [
      chat.id,
      currentActor,
      isTogglingReaction,
      persistedMessageIdSet,
      toggleReaction,
    ],
  );

  if (messages.length === 0) {
    return (
      <ConversationEmptyState
        description="Start the conversation below. Replies will refresh automatically."
        title="No messages yet"
      />
    );
  }

  return (
    <Conversation
      computeItemKey={(_index, message) => message.id}
      data={messages}
      footerHeight={256}
      scrollButtonClassName="bottom-52"
      itemContent={(index, message) => (
        <ChatMessageRow
          agents={agents}
          chat={chat}
          currentActor={currentActor}
          isTogglingReaction={isTogglingReaction}
          message={message}
          nextMessage={
            index < messages.length - 1 ? messages[index + 1] : undefined
          }
          onToggleReaction={updateMessageReaction}
          persisted={persistedMessageIdSet.has(message.id)}
          previousMessage={index > 0 ? messages[index - 1] : undefined}
          selfUserId={selfUserId}
          userName={userName}
          usersById={usersById}
        />
      )}
    />
  );
}
