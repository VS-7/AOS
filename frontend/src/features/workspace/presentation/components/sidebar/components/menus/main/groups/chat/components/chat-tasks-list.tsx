import * as React from "react";
import { SidebarMenu } from "@/components/ui/sidebar";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { ChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import { ChatRow } from "./chat-row";
import { t } from "@/lib/i18n";

interface ChatTasksListProps {
  chats: Chat[];
  agentIds: ReadonlySet<string>;
  currentChatId?: string;
}

/**
 * Tasks tab — chats linked via `chat.task`.
 */
export function ChatTasksList({
  chats,
  agentIds,
  currentChatId,
}: ChatTasksListProps) {
  const rows = React.useMemo(
    () => ChatKindHelper.filterByKind(chats, "task", agentIds),
    [agentIds, chats],
  );

  if (rows.length === 0) {
    return (
      <p className="px-2 py-3 text-xs text-muted-foreground/70">
        {t("No task chats yet")}
      </p>
    );
  }

  return (
    <SidebarMenu>
      {rows.map((chat, index) => (
        <ChatRow
          key={chat.id}
          chat={chat}
          kind="task"
          isActive={currentChatId === chat.id}
          index={index}
          subtitle={chat.task}
        />
      ))}
    </SidebarMenu>
  );
}
