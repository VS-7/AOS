import * as React from "react";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Chat, Message } from "@/features/chat/interfaces/chat.interfaces";
import { ChatMessageItem } from "@/features/chat/presentation/components/chat-message-item";
import { Conversation, ConversationEmptyState } from "@/components/ui/conversation";
import { AgentToolThinkingHelper } from "@/features/agent/presentation/helpers/agent-tool-thinking.helper";
import { ChatThreadHelper } from "@/features/chat/presentation/helpers/chat-thread.helper";

interface ChatMessageListProps {
  agents: Agent[];
  chat: Chat;
  persistedMessageIds?: string[];
  selfUserId?: string;
  userName: string;
}

/** Whether a message has anything worth rendering — text, a tool call, a file, or agent thinking. */
function canRenderMessage(message: Message): boolean {
  const parts = message.parts ?? [];
  const hasText = parts.some(
    (part) => part.type === "text" && (part.text?.trim().length ?? 0) > 0 && !part.text?.startsWith("[system-reminder]:"),
  );
  return (
    hasText ||
    parts.some((part) => part.type === "tool-call") ||
    parts.some((part) => part.type === "file") ||
    AgentToolThinkingHelper.toThinkingSteps(message).length > 0
  );
}

export function ChatMessageList({ agents, chat, persistedMessageIds = [], selfUserId, userName }: ChatMessageListProps) {
  const messages = React.useMemo(() => (chat.messages ?? []).filter(canRenderMessage), [chat.messages]);
  const persistedMessageIdSet = React.useMemo(() => new Set(persistedMessageIds), [persistedMessageIds]);

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
      itemContent={(index, message) => {
        const previousMessage = index > 0 ? messages[index - 1] : undefined;
        const timestamp = ChatThreadHelper.getMessageTimestamp(message);
        const previousTimestamp = previousMessage ? ChatThreadHelper.getMessageTimestamp(previousMessage) : null;
        const showDayDivider = !ChatThreadHelper.isSameDay(timestamp, previousTimestamp);

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
              previousMessage={previousMessage}
              selfUserId={selfUserId}
              userName={userName}
              animateIn={!persistedMessageIdSet.has(message.id)}
            />
          </React.Fragment>
        );
      }}
    />
  );
}
