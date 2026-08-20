import * as React from "react";
import { Loading03Icon } from "@hugeicons/core-free-icons";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuMotionItem,
} from "@/components/ui/sidebar";
import { aos } from "@/app/aos";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import {
  ChatKindHelper,
  type ChatKind,
} from "@/features/chat/services/chat/chat-kind.helper";
import { ChatActivityStamp } from "./chat-activity-stamp";
import { ChatRowKindIcon } from "./chat-row-kind-icon";

interface ChatRowProps {
  chat: Chat;
  kind: ChatKind;
  isActive: boolean;
  index: number;
  subtitle?: string;
  /** Prefer the processing spinner as the leading glyph (Live subsection). */
  preferSpinner?: boolean;
}

/**
 * Shared sidebar row for Live / Tasks / Runs chat lists.
 */
export function ChatRow({
  chat,
  kind,
  isActive,
  index,
  subtitle,
  preferSpinner = false,
}: ChatRowProps) {
  const isProcessing = aos.stores.agent.useState((s) =>
    ChatKindHelper.isProcessing(chat.id, s.occupancy),
  );

  const showSpinner = preferSpinner || isProcessing;

  return (
    <SidebarMenuMotionItem index={index}>
      <SidebarMenuItem>
        <SidebarMenuButton
          isActive={isActive}
          data-active={isActive}
          className="group/chat-row"
          onClick={() =>
            openChatTab({
              chatId: chat.id,
              title: chat.title || chat.id,
            })
          }
        >
          <span className="relative flex size-5 shrink-0 items-center justify-center">
            {showSpinner ? (
              <HugeiconsIcon
                icon={Loading03Icon}
                className="size-3.5 animate-spin text-amber-500"
              />
            ) : (
              <ChatRowKindIcon kind={kind} />
            )}
          </span>
          <span className="min-w-0 flex-1 truncate">{chat.title || chat.id}</span>
          {subtitle ? (
            <span className="max-w-[4.5rem] truncate text-[10px] text-muted-foreground/70">
              {subtitle}
            </span>
          ) : null}
          <ChatActivityStamp at={chat.updatedAt} />
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenuMotionItem>
  );
}
