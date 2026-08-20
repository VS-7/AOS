import * as React from "react";
import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { openChatTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";

/**
 * Deep-link entry for `/chats/$id`.
 *
 * Promotes the thread into a workspace `chat` viewport tab (ChatPanel) so
 * multi-task tabs stay the primary surface. Does not mount chat UI here —
 * avoids dual `useChat` with ChatPanel.
 */
export const ChatPage = aos
  .page("/chats/$id")
  .withMetadata({
    title: "Chat",
    description: "Slack-style chat workspace",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ request }) => {
    return {
      chatId: request.params.id,
    };
  })
  .withComponent(({ route }) => {
    const { chatId } = route.useLoaderData();

    React.useEffect(() => {
      openChatTab({ chatId });
    }, [chatId]);

    return null;
  })
  .build();
