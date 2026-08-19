import { AosTriggerGroup } from "@/app/builders/trigger";

export const viewportGroup = AosTriggerGroup.create("Viewport")
  .withOrder(3)
  .addTrigger({
    id: "viewport.toggle.sidebar",
    label: "Toggle Sidebar",
    keybind: "mod+b",
    icon: "PanelLeft",
    handler: ({ stores }) => stores.viewport.actions.toggle('layout.sidebar.visible'),
  })
  .addTrigger({
    id: "viewport.toggle.page_sidebar",
    label: "Toggle Page Sidebar",
    keybind: "mod+shift+b",
    icon: "PanelLeftClose",
    handler: ({ stores }) => stores.viewport.actions.toggle('page.sidebar.visible'),
  })
  .addTrigger({
    id: "viewport.toggle.details",
    label: "Toggle Details Panel",
    keybind: "mod+shift+d",
    icon: "PanelRight",
    handler: ({ stores }) => stores.viewport.actions.toggle('page.details.visible'),
  })
  .addTrigger({
    id: "viewport.toggle.inbox",
    label: "Toggle Inbox",
    keybind: "mod+shift+i",
    icon: "Inbox",
    hidden: true,
    handler: ({ stores }) => stores.viewport.actions.toggle('inbox.panel.visible'),
  })
  .addTrigger({
    id: "viewport.fullscreen.page",
    label: "Toggle Page Fullscreen",
    keybind: "mod+shift+f",
    icon: "Fullscreen",
    handler: ({ stores }) => {
      const isFocused = stores.viewport.state.fullscreen === 'page';
      stores.viewport.actions.fullscreen(isFocused ? undefined : 'page');
    },
  });
