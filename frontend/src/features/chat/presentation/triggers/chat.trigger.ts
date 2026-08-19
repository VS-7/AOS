import { AosTriggerGroup } from "@/app/builders/trigger";

export const chatGroup = AosTriggerGroup.create("Chat")
  .withOrder(2)
  .addTrigger({
    id: "agent.chat.new",
    label: "New Chat",
    keybind: "mod+shift+n",
    icon: "MessageSquarePlus",
    handler: ({ stores }) => {
      stores.viewport.actions.toggle('agent.panel', true);
      stores.chat.actions.create();
    },
  })
  .addTrigger({
    id: "agent.chat.history",
    label: "Toggle Chat History",
    keybind: "mod+shift+h",
    icon: "History",
    handler: ({ stores }) => {
      stores.viewport.actions.toggle('agent.panel', true);
      stores.viewport.actions.toggle('agent.history.visible');
    },
  });

