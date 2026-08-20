import * as React from "react";
import { SidebarMenu, SidebarMenuMotionItem } from "@/components/ui/sidebar";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { ChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import { ChannelItem } from "../../channels/components/channel-item";

interface ChatChannelsListProps {
  chats: Chat[];
  agentIds: ReadonlySet<string>;
  currentChatId?: string;
  onChanged?: () => void;
}

/**
 * Channels tab — free workspace channels (excludes agent/task/run/external).
 */
export function ChatChannelsList({
  chats,
  agentIds,
  currentChatId,
  onChanged,
}: ChatChannelsListProps) {
  const channelChats = React.useMemo(
    () => ChatKindHelper.filterByKind(chats, "channel", agentIds),
    [agentIds, chats],
  );

  if (channelChats.length === 0) {
    return (
      <p className="px-2 py-3 text-xs text-muted-foreground/70">
        No channels yet
      </p>
    );
  }

  return (
    <SidebarMenu>
      {channelChats.map((chat, index) => (
        <SidebarMenuMotionItem key={chat.id} index={index}>
          <ChannelItem
            chat={chat}
            isActive={currentChatId === chat.id}
            onChanged={onChanged}
          />
        </SidebarMenuMotionItem>
      ))}
    </SidebarMenu>
  );
}
