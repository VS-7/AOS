import { AosTriggerGroup } from "@/app/builders/trigger";
import z from "zod";
import type { ViewportTabState } from "../stores/viewport.store";
import { normalizeBrowserUrl } from "../stores/browser.store";

export const tabsGroup = AosTriggerGroup.create("Tabs")
  .withOrder(0) // Priorities tabs
  .withMetadataSchema(z.object({
    tabId: z.string(),
    type: z.enum(['in-app', 'browser', 'file']),
    favicon: z.string().optional().nullable(),
    active: z.boolean()
  }))
  .addTrigger({
    id: "tabs.newTab",
    label: "New Browser Tab",
    keybind: "mod+t",
    icon: "Plus",
    handler: ({ stores }) => {
      stores.viewport.actions.createTab();
    },
  })
  .addTrigger({
    id: "tabs.closeTab",
    label: "Close Browser Tab",
    keybind: "mod+w",
    icon: "X",
    handler: ({ stores }) => {
      const activeId = stores.viewport.state.tabs.current;
      if (activeId === 'aos') return;
      stores.viewport.actions.closeTab(activeId);
    },
  })
  .addTrigger({
    id: "workspace.files",
    label: "View Files",
    icon: "Folder",
    handler: ({ stores, response }) => {
      stores.viewport.actions.setSidebarMenu('files')
      response.redirect("/")
    },
  })
  .addTrigger({
    id: "tabs.reload",
    label: "Reload",
    keybind: "mod+r",
    icon: "RefreshCw",
    handler: ({ stores }) => {
      const activeId = stores.viewport.state.tabs.current;
      const tab = stores.viewport.state.tabs.items.find((t: ViewportTabState) => t.id === activeId);
      if (tab?.type === 'browser') {
        window.aos?.browser?.reload({ tabId: activeId });
      } else {
        window.location.reload();
      }
    },
  })
  .addTrigger({
    id: "tabs.back",
    label: "Go Back",
    keybind: "mod+left",
    icon: "ChevronLeft",
    handler: ({ stores }) => {
      const activeId = stores.viewport.state.tabs.current;
      const tab = stores.viewport.state.tabs.items.find((t: ViewportTabState) => t.id === activeId);
      if (tab?.type === 'browser') {
        window.aos?.browser?.goBack({ tabId: activeId });
      } else {
        // Could implement app navigation back if needed
        window.history.back();
      }
    },
  })
  .addTrigger({
    id: "tabs.forward",
    label: "Go Forward",
    keybind: "mod+right",
    icon: "ChevronRight",
    handler: ({ stores }) => {
      const activeId = stores.viewport.state.tabs.current;
      const tab = stores.viewport.state.tabs.items.find((t: ViewportTabState) => t.id === activeId);
      if (tab?.type === 'browser') {
        window.aos?.browser?.goForward({ tabId: activeId });
      } else {
        // Could implement app navigation forward if needed
        window.history.forward();
      }
    },
  })
  .addTrigger({
    id: "tabs.focusAddress",
    label: "Focus Address Bar",
    keybind: "mod+l",
    icon: "Search",
    handler: ({ stores }) => {
      const activeId = stores.viewport.state.tabs.current;
      const tab = stores.viewport.state.tabs.items.find((t: ViewportTabState) => t.id === activeId);
      if (tab?.type === 'browser') {
        stores.browser.actions.focusAddressBar();
      }
    },
  })
  .addTrigger({
    id: "tabs.navigate",
    label: "Navigate",
    icon: "Globe",
    hidden: true,
    schema: z.object({
      url: z.string().min(1),
    }),
    handler: ({ stores, input }) => {
      const activeId = stores.viewport.state.tabs.current;
      const tab = stores.viewport.state.tabs.items.find((t: ViewportTabState) => t.id === activeId);
      if (tab?.type === 'browser') {
        // `input` doesn't pick up this trigger's own `schema` shape through
        // the builder's generics — same kind of gap as `aos.useForm`'s.
        const url = normalizeBrowserUrl((input as { url: string }).url);
        stores.viewport.actions.updateTab(activeId, { url, isLoading: true });
        window.aos?.browser?.navigate({ tabId: activeId, url });
      }
    },
  })
  .addTrigger({
    id: "app.commander.open",
    label: "Toggle Commander",
    keybind: "mod+k",
    hidden: true,
    handler: ({ stores }) => stores.viewport.actions.toggleCommander(),
  })
