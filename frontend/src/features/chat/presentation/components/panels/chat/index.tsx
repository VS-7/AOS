import { cn } from "@/lib/utils";
import { aos } from "@/app/aos";
import { ChatContent } from "@/features/chat/presentation/components/chat-content";
import { getTabChatId } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import type { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

/**
 * Viewport panel that keep-alive mounts every open `chat` tab and only
 * visually hides inactive ones, preserving React state and subscriptions.
 */
export function ChatPanel() {
  const activeTabId = aos.stores.viewport.useState(
    (state) => state.tabs.current,
  );
  const chatTabs = aos.stores.viewport.useState((state) =>
    state.tabs.items.filter((tab) => tab.type === "chat"),
  );
  const activeTab = aos.stores.viewport.useState((state) =>
    state.tabs.items.find((tab) => tab.id === activeTabId),
  );
  const isChatSurfaceActive = activeTab?.type === "chat";

  if (chatTabs.length === 0) return null;

  return (
    <div
      className={cn(
        "relative flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
        !isChatSurfaceActive && "hidden",
      )}
    >
      {chatTabs.map((tab) => (
        <ChatTabSurface
          key={tab.id}
          tab={tab}
          active={tab.id === activeTabId}
        />
      ))}
    </div>
  );
}

function ChatTabSurface({
  tab,
  active,
}: {
  tab: ViewportTabState;
  active: boolean;
}) {
  const chatId = getTabChatId(tab);

  return (
    <div
      className={cn(
        "absolute inset-0 flex min-h-0 min-w-0 flex-col overflow-hidden",
        !active && "hidden",
      )}
    >
      {chatId ? (
        <ChatContent chatId={chatId} />
      ) : (
        <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
          Missing chat id for this tab.
        </div>
      )}
    </div>
  );
}
