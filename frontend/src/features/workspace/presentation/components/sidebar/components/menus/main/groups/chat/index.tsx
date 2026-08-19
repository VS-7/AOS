import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
} from "@/components/ui/sidebar";
import { aos } from "@/app/aos";
import { FractalChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import { resolveActiveChatId } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { CreateChannelDialog } from "../channels/components/create-channel-dialog";
import { ChatChannelsList } from "./components/chat-channels-list";
import { ChatRunsList } from "./components/chat-runs-list";
import {
  ChatSearch,
  ChatSearchToggle,
} from "./components/chat-search";
import {
  ChatTabs,
  readStoredChatTab,
  storeChatTab,
  type ChatSidebarTab,
} from "./components/chat-tabs";
import { ChatTasksList } from "./components/chat-tasks-list";
import { ChatTeamList } from "./components/chat-team-list";

/**
 * Workspace Chat group — channels, team, tasks, and runs with a Live strip.
 *
 * Opens chats as viewport tabs via openChatTab. Active highlight follows
 * the focused chat tab metadata. Finder searches across all kinds.
 */
export function WorkspaceSidebarChatGroupMenu() {
  const activeTab = aos.stores.viewport.useState((s) =>
    s.tabs.items.find((tab) => tab.id === s.tabs.current),
  );
  const currentChatId = resolveActiveChatId(activeTab);

  const [tab, setTab] = React.useState<ChatSidebarTab>(() => readStoredChatTab());
  const [searchOpen, setSearchOpen] = React.useState(false);

  const chats = aos.stores.chat.useState((s) => s.items);
  const agents = aos.stores.agent.useState((s) => s.items);
  const occupancy = aos.stores.agent.useState((s) => s.occupancy);

  const agentIds = React.useMemo(
    () => new Set(agents.map((agent) => agent.id)),
    [agents],
  );

  React.useEffect(() => {
    void aos.stores.chat.actions.refresh();
  }, []);

  const handleTabChange = React.useCallback((next: ChatSidebarTab) => {
    setTab(next);
    storeChatTab(next);
  }, []);

  const refreshLists = React.useCallback(() => {
    void aos.stores.chat.actions.refresh();
  }, []);

  const badges = React.useMemo(
    () => ({
      channels: FractalChatKindHelper.countProcessingByKind(
        chats,
        "channel",
        occupancy,
        agentIds,
      ),
      team: agents.filter((agent) =>
        Object.values(occupancy).some((ids) => ids.includes(agent.id)),
      ).length,
      tasks: FractalChatKindHelper.countProcessingByKind(
        chats,
        "task",
        occupancy,
        agentIds,
      ),
      runs: FractalChatKindHelper.countProcessingByKind(
        chats,
        "run",
        occupancy,
        agentIds,
      ),
    }),
    [agentIds, agents, chats, occupancy],
  );

  return (
    <SidebarGroup className="relative">
      <div className="flex h-8 items-center gap-1 px-2">
        <SidebarGroupLabel className="flex-1 px-0">Chat</SidebarGroupLabel>
        <ChatSearchToggle open={searchOpen} onOpenChange={setSearchOpen} />
        {tab === "channels" && !searchOpen ? <CreateChannelDialog /> : null}
      </div>

      <SidebarGroupContent className="gap-1">
        <ChatSearch
          open={searchOpen}
          onOpenChange={setSearchOpen}
          chats={chats}
          agents={agents}
          agentIds={agentIds}
          currentChatId={currentChatId}
        />

        <AnimatePresence initial={false} mode="popLayout">
          {!searchOpen ? (
            <motion.div
              key="chat-browse"
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.16, ease: [0.22, 1, 0.36, 1] }}
              className="space-y-1"
            >
              <ChatTabs
                value={tab}
                onValueChange={handleTabChange}
                badges={badges}
              />

              {tab === "channels" ? (
                <ChatChannelsList
                  chats={chats}
                  agentIds={agentIds}
                  currentChatId={currentChatId}
                  onChanged={refreshLists}
                />
              ) : null}
              {tab === "team" ? (
                <ChatTeamList agents={agents} currentChatId={currentChatId} />
              ) : null}
              {tab === "tasks" ? (
                <ChatTasksList
                  chats={chats}
                  agentIds={agentIds}
                  currentChatId={currentChatId}
                />
              ) : null}
              {tab === "runs" ? (
                <ChatRunsList
                  chats={chats}
                  agentIds={agentIds}
                  currentChatId={currentChatId}
                />
              ) : null}
            </motion.div>
          ) : null}
        </AnimatePresence>
      </SidebarGroupContent>
    </SidebarGroup>
  );
}
