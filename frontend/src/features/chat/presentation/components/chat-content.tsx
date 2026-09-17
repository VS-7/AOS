import { Page, PageBody, PageHeader } from "@/components/ui/page";
import { aos } from "@/app/aos";
import { AvatarAgentFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { AlertTriangle, MessageSquareOff } from "lucide-react";
import * as React from "react";
import { useChat } from "@/features/chat/presentation/hooks/use-chat";
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper";
import {
  closeChatTab,
  getTabChatId,
  syncChatTabTitle,
} from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { ChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import { ChatComposer } from "@/features/chat/presentation/pages/($id)/components/chat-composer.component";
import { ChatMessageList } from "@/features/chat/presentation/pages/($id)/components/chat-message-list.component";
import { ChatRowKindIcon } from "@/features/workspace/presentation/components/sidebar/components/menus/main/groups/chat/components/chat-row-kind-icon";
import { ChatActionsMenu } from "./chat-actions-menu";
import { t } from "@/lib/i18n";

interface ChatContentProps {
  chatId: string;
  userName?: string;
  userId?: string;
}

/** The daemon's answer for an id this workspace has no conversation under. */
const CHAT_NOT_FOUND = "AOS_CHAT_NOT_FOUND";

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
    t("You");
  const resolvedUserId = userId ?? authUser?.id ?? "user";

  const usersById = React.useMemo(() => {
    const map = new Map(
      directoryUsers.map((user) => [user.id, user] as const),
    );

    // The session profile wins over the directory's copy of the viewer, so
    // their own messages carry the name they signed in with rather than
    // falling back to the installation's `config.user`.
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

  // Once the daemon has said the conversation does not exist, stop asking.
  // Every chat write in the workspace invalidates every chat read, so an open
  // tab for a deleted conversation refetched it — and was refused — each time
  // anything was said anywhere.
  const [notFound, setNotFound] = React.useState(false);
  const liveChat = useChat({ chatId, enabled: !notFound });
  const errorCode = liveChat.error?.code;
  React.useEffect(() => {
    if (errorCode === CHAT_NOT_FOUND) setNotFound(true);
  }, [errorCode]);

  const chat = liveChat.chat;
  const agentIds = React.useMemo(
    () => new Set(agents.map((agent) => agent.id)),
    [agents],
  );

  const isAgentDirectMessage = chat
    ? ComposerHelper.isAgentDirectMessage(chat, agents)
    : false;
  const directAgent =
    chat && isAgentDirectMessage
      ? agents.find(
          (agent) =>
            agent.id === chat.id ||
            (chat.participants ?? []).some(
              (participant) =>
                participant.type === "agent" && participant.id === agent.id,
            ),
        )
      : undefined;
  const kind = chat ? ChatKindHelper.classify(chat, agentIds) : undefined;
  const displayTitle = directAgent ? directAgent.name : chat?.title;

  // The header names the conversation, and its tab says the same. Whoever
  // opens the tab passes a title of their own (search passes the stored
  // one, a task its name), so the tab's title is watched too: a later open
  // with another name is put back instead of staying until the header's
  // title itself changes.
  const tabTitle = aos.stores.viewport.useState(
    (state) =>
      state.tabs.items.find(
        (tab) => tab.type === "chat" && getTabChatId(tab) === chatId,
      )?.title,
  );
  React.useEffect(() => {
    if (notFound) {
      syncChatTabTitle(chatId, t("Conversation not found"));
    } else if (displayTitle) {
      syncChatTabTitle(chatId, displayTitle);
    }
  }, [chatId, displayTitle, notFound, tabTitle]);

  if (notFound) {
    return (
      <ChatUnavailable
        icon={<MessageSquareOff className="size-5 text-muted-foreground" />}
        title={t("This conversation no longer exists")}
        description={t(
          "It was deleted, or this link points to a conversation this workspace does not have.",
        )}
        action={
          <Button type="button" variant="outline" size="sm" onClick={() => closeChatTab(chatId)}>
            {t("Close tab")}
          </Button>
        }
      />
    );
  }

  if (!chat) {
    if (!liveChat.error) return null;
    return (
      <ChatUnavailable
        icon={<AlertTriangle className="size-5 text-destructive" />}
        title={t("This conversation could not be opened")}
        description={liveChat.error.message}
        action={
          <Button type="button" variant="outline" size="sm" onClick={() => liveChat.refresh()}>
            {t("Try again")}
          </Button>
        }
      />
    );
  }

  return (
    <Page>
      <PageHeader>
        <div className="flex w-full min-w-0 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {directAgent ? (
              <AvatarAgentFallback size={20} name={directAgent.id} />
            ) : (
              <ChatRowKindIcon kind={kind ?? "channel"} />
            )}

            <div className="min-w-0 flex items-center">
              <h1 className="text-xs font-semibold text-foreground leading-3">
                {displayTitle}
              </h1>
            </div>
          </div>
          <ChatActionsMenu
            chat={chat}
            canRename={!directAgent}
            onCleared={liveChat.replaceAndRefresh}
          />
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
          kind={kind}
          onCleared={liveChat.replaceAndRefresh}
          onConfirmed={liveChat.replaceMessage}
          onFailed={liveChat.removeMessage}
          onSent={liveChat.appendMessage}
          userId={resolvedUserId}
        />
      </PageBody>
    </Page>
  );
}

/** A conversation that cannot be shown, and what can be done about it. */
function ChatUnavailable({
  icon,
  title,
  description,
  action,
}: {
  icon: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div role="status" className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
      {icon}
      <div className="max-w-sm space-y-1">
        <h2 className="text-sm font-medium text-foreground">{title}</h2>
        {description ? (
          <p className="text-sm break-words text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {action}
    </div>
  );
}
