import * as React from "react";
import { useQuery } from "@tanstack/react-query";
import { Page, PageBody, PageHeader } from "@/components/ui/page";
import { AvatarAgentFallback } from "@/components/ui/avatar";
import { Hash } from "lucide-react";
import { client } from "@/lib/client";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import { useChat } from "@/features/chat/presentation/hooks/use-chat";
import { ChatMessageList } from "@/features/chat/presentation/components/chat-message-list";
import { ChatComposer } from "@/features/chat/presentation/components/composer/chat-composer";
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper";
import type { StreamingAnswer } from "@/lib/realtime";

interface ChatContentProps {
  chatId: string;
  userName?: string;
}

/**
 * Shared chat surface — ported from the original's ChatContent. That
 * component reads self identity from an auth session and a workspace
 * directory of human users; AOS has neither built yet (single-operator
 * model, see ChatThreadHelper), so `userName` defaults straight to "You"
 * instead of resolving through a directory that doesn't exist.
 */
export function ChatContent({ chatId, userName = "You" }: ChatContentProps) {
  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: async () =>
      (await client.invoke("agents_list", { _reasoning: "the chat screen is open" })) as { agents: Agent[] },
  });
  const agents = agentsQuery.data?.agents ?? [];

  const liveChat = useChat({ chatId });
  const chat = liveChat.chat;

  const streaming = useQuery<StreamingAnswer>({
    queryKey: ["chat", chatId, "streaming"],
    queryFn: () => ({ text: "", reasoning: "" }),
    enabled: chatId !== "",
    staleTime: Infinity,
  });

  const directAgent = React.useMemo(() => {
    if (!chat) return undefined;
    return agents.find(
      (agent) =>
        agent.id === chat.id ||
        (chat.participants ?? []).some((p) => p.type === "agent" && p.id === agent.id),
    );
  }, [agents, chat]);

  if (liveChat.isLoading) {
    return <p className="empty">Reading the conversation…</p>;
  }
  if (!chat) {
    return null;
  }

  return (
    <Page>
      <PageHeader>
        <div className="flex w-full min-w-0 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {directAgent ? (
              <AvatarAgentFallback size={20} name={directAgent.id} />
            ) : (
              <Hash className="size-3.5 text-muted-foreground" />
            )}
            <div className="min-w-0 flex items-center">
              <h1 className="text-xs font-semibold text-foreground leading-3">
                {directAgent ? directAgent.name : chat.title}
              </h1>
            </div>
          </div>
        </div>
      </PageHeader>
      <PageBody className="min-h-0 overflow-hidden overflow-y-hidden relative flex flex-col gap-3">
        <div className="min-h-0 flex-1 overflow-hidden pb-24">
          <ChatMessageList agents={agents} chat={chat} userName={userName} />
        </div>

        {streaming.data?.text ? (
          <div className="px-6 text-sm text-muted-foreground">{streaming.data.text}</div>
        ) : null}

        <ChatComposer
          agents={agents}
          chat={chat}
          isDirectMessage={ComposerHelper.isAgentDirectMessage(chat)}
        />
      </PageBody>
    </Page>
  );
}
