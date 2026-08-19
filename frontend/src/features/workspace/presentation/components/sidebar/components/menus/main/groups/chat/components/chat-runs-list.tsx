import * as React from "react";
import { SidebarMenu } from "@/components/ui/sidebar";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { FractalChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import { ChatRow } from "./chat-row";

interface ChatRunsListProps {
  chats: Chat[];
  agentIds: ReadonlySet<string>;
  currentChatId?: string;
}

/**
 * Runs tab — chats linked via `chat.routine`.
 */
export function ChatRunsList({
  chats,
  agentIds,
  currentChatId,
}: ChatRunsListProps) {
  const rows = React.useMemo(
    () => FractalChatKindHelper.filterByKind(chats, "run", agentIds),
    [agentIds, chats],
  );

  if (rows.length === 0) {
    return (
      <p className="px-2 py-3 text-xs text-muted-foreground/70">
        No routine runs yet
      </p>
    );
  }

  return (
    <SidebarMenu>
      {rows.map((chat, index) => (
        <ChatRow
          key={chat.id}
          chat={chat}
          kind="run"
          isActive={currentChatId === chat.id}
          index={index}
          subtitle={chat.routine}
        />
      ))}
    </SidebarMenu>
  );
}
