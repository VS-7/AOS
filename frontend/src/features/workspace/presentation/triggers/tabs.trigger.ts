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
        // Bumps the iframe renderer's key (renderer.component.tsx) so it
        // remounts — an unchanged src does not re-fetch on its own, and
        // there is no native embed to ask for a reload instead.
        stores.viewport.actions.updateTab(activeId, { reloadNonce: (tab.reloadNonce ?? 0) + 1 });
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
        // Not implemented: an iframe exposes no navigation history to query
        // or step through, and there is no native embed bridge (window.d.ts's
        // own doc) to ask for one instead — see browser/index.tsx's comment.
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
        // Not implemented — see tabs.back's own comment just above.
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
        // The iframe renderer (renderer.component.tsx) picks up the new url
        // reactively — no bridge call needed, unlike the Electron-only
        // `window.aos.browser.navigate` this used to also call, which was
        // always a no-op (window.d.ts documents that bridge as permanently
        // undefined here).
        const url = normalizeBrowserUrl((input as { url: string }).url);
        stores.viewport.actions.updateTab(activeId, { url, status: "loading" });
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
