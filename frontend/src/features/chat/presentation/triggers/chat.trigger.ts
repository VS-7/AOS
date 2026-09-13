import { AosTriggerGroup } from "@/app/builders/trigger";
import { openAgentDmTab } from "@/features/chat/presentation/helpers/open-chat-tab.helper";
import { t } from "@/lib/i18n";

/**
 * The palette's chat commands.
 *
 * "New Chat" used to open an agent side panel this build does not have
 * (`toggle('agent.panel', true)` even replaced the `{visible}` object with a
 * boolean) and call `stores.chat.actions.create`, which does not exist — the
 * TypeError was swallowed and the palette just closed. A chat here is a tab,
 * and the conversation a person starts without naming anyone is with the
 * workspace's copilot, so that is what it opens: the existing DM when there
 * is one, a new one otherwise.
 *
 * "Toggle Chat History" is gone rather than fixed: it flipped a flag nothing
 * renders.
 */
export const chatGroup = AosTriggerGroup.create("Chat")
  .withOrder(2)
  .addTrigger({
    id: "agent.chat.new",
    label: "New Chat",
    keybind: "mod+shift+n",
    icon: "MessageSquarePlus",
    handler: async ({ stores }) => {
      const agents: Array<{ id: string; name?: string; orchestrator?: boolean }> = stores.agent.state.items ?? [];
      const copilot = agents.find((agent) => agent.orchestrator);
      if (!copilot) {
        throw new Error(t("This workspace has no copilot agent to chat with."));
      }
      await openAgentDmTab({ agentId: copilot.id, title: copilot.name || copilot.id });
    },
  });
