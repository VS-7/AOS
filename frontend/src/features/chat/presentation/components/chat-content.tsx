import { Page, PageBody, PageHeader } from "@/components/ui/page";
import { aos } from "@/app/aos";
import { AvatarAgentFallback } from "@/components/ui/avatar";
import { Hash } from "lucide-react";
import * as React from "react";
import { useChat } from "@/features/chat/presentation/hooks/use-chat";
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper";
import { ChatComposer } from "@/features/chat/presentation/pages/($id)/components/chat-composer.component";
import { ChatMessageList } from "@/features/chat/presentation/pages/($id)/components/chat-message-list.component";

interface ChatContentProps {
  chatId: string;
  userName?: string;
  userId?: string;
}

/**
 * Shared chat surface used by the `/chats/$id` page and the viewport chat panel.
 *
 * Self identity always comes from the auth session — never from
 * `config.user` (that is the installation owner profile, not the logged-in member).
 */
export function ChatContent({ chatId, userName, userId }: ChatContentProps) {
  const { items: agents } = aos.stores.agent.useState();
  const directoryUsers = aos.stores.workspace.useState(
    (s) => s.directory?.users ?? [],
  );
  const authUser = aos.stores.auth.useState((s) => s.user);

  const resolvedUserName =
    userName ??
    authUser?.name?.trim() ??
    authUser?.username?.trim() ??
    "You";
  const resolvedUserId = userId ?? authUser?.id ?? "user";

  const usersById = React.useMemo(() => {
    const map = new Map(
      directoryUsers.map((user) => [user.id, user] as const),
    );

    // Directory is viewer-relative (self excluded). Re-inject the session
    // profile so own messages resolve name/image without falling back to
    // installation `config.user`.
    if (authUser?.id) {
      map.set(authUser.id, {
        id: authUser.id,
        name: authUser.name,
        username: authUser.username,
        email: authUser.email,
        image: authUser.image,
      });
    }

    return map;
  }, [authUser, directoryUsers]);

  const liveChat = useChat({ chatId });
  const chat = liveChat.chat;
  if (!chat) return null;

  const isAgentDirectMessage = ComposerHelper.isAgentDirectMessage(
    chat,
    agents,
  );
  const directAgent = isAgentDirectMessage
    ? agents.find(
        (agent) =>
          agent.id === chat.id ||
          (chat.participants ?? []).some(
            (participant) =>
              participant.type === "agent" && participant.id === agent.id,
          ),
      )
    : undefined;

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
      <PageBody className="min-h-0 overflow-hidden overflow-y-hidden relative">
        <ChatMessageList
          agents={agents}
          chat={chat}
          isRefreshing={liveChat.isRefreshing}
          onReactionToggled={liveChat.refresh}
          persistedMessageIds={liveChat.persistedMessageIds}
          selfUserId={authUser?.id}
          userName={resolvedUserName}
          usersById={usersById}
        />
        <ChatComposer
          agents={agents}
          chat={chat}
          isDirectMessage={isAgentDirectMessage}
          onConfirmed={liveChat.replaceMessage}
          onFailed={liveChat.removeMessage}
          onSent={liveChat.appendMessage}
          userId={resolvedUserId}
        />
      </PageBody>
    </Page>
  );
}
