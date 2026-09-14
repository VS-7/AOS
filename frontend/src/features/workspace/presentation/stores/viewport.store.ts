import { generateId } from "ai";
import { arrayMove } from "@dnd-kit/sortable";
import { AosStore } from "@/app/builders/store";
import {
  DEFAULT_SETTINGS_SECTION,
  type SettingsSectionId,
} from "@/features/workspace/presentation/components/settings/constants";
import { SettingsRouteHelper } from "@/features/workspace/presentation/helpers/settings-route.helper";

export type ViewportTabType = "in-app" | "browser" | "file" | "changes" | "chat";
export type WorkspaceSidebarMenu = "main" | "files" | "settings";

/**
 * What a settings section should open on, when the caller knows.
 *
 * Carried in the URL (`?agent=luara`), so the section reads it the same way
 * whether it was opened from a hover card or from a link.
 */
export interface SettingsSelection {
  agent?: string;
}

async function navigateToSettingsSection(
  sectionId: SettingsSectionId,
  selection?: SettingsSelection,
): Promise<void> {
  const { router } = await import("@/app/router");
  const args = SettingsRouteHelper.sectionIdToNavigateArgs(sectionId);
  await router.navigate(
    (selection
      ? {
          ...args,
          // Merged into what is already there: the window's own parameters
          // (`daemon`, `platform`) ride in the same query string.
          search: (previous: Record<string, unknown>) => ({ ...previous, ...selection }),
        }
      : args) as never,
  );
}

export interface ViewportTabState {
  id: string;
  type: ViewportTabType;
  title: string;
  url?: string;
  status?: "loading" | "idle";
  canGoBack?: boolean;
  canGoForward?: boolean;
  favicon?: string | null;
  error?: string | null;
  closable?: boolean;
  metadata?: Record<string, string | number | boolean>;
  /**
   * Bumped by the `tabs.reload` trigger to force the iframe-based browser
   * renderer to remount — an unchanged `src` does not re-fetch on its own,
   * and there is no native embed (see window.d.ts's own doc on why) to ask
   * for a reload instead.
   */
  reloadNonce?: number;
}

export interface ViewportVisibilityState {
  layout: {
    sidebar: { visible: boolean; menu: WorkspaceSidebarMenu; width: number };
  };
  page: {
    sidebar: { visible: boolean; enabled: boolean };
    details: { visible: boolean; enabled: boolean };
  };
  agent: { history: { visible: boolean }; panel: { visible: boolean } };
  inbox: { panel: { visible: boolean } };
  tasks: { dialog: { visible: boolean; project?: string } };
  project: { dialog: { visible: boolean } };
  goal: { dialog: { visible: boolean } };
  settings: { dialog: { visible: boolean; section: SettingsSectionId } };
}

const INITIAL_APP_TAB: ViewportTabState = {
  id: "aos",
  type: "in-app",
  title: "AOS",
  closable: false,
  metadata: {
    hasPageSidebar: false,
    hasPageDetails: false,
  },
};

export const ViewportStore = AosStore.create("viewport")
  .withState({
    fullscreen: undefined as undefined | "chat" | "page",
    layout: { sidebar: { visible: true, menu: "main", width: 256 } },
    page: {
      sidebar: { visible: true, enabled: false },
      details: { visible: true, enabled: false },
    },
    agent: { history: { visible: false }, panel: { visible: true } },
    inbox: { panel: { visible: false } },
    // `project`: what the create-task dialog files the new task under — set
    // by whoever opens it for a project (a project's Tasks tab), cleared when
    // it closes.
    tasks: { dialog: { visible: false, project: undefined as string | undefined } },
    project: { dialog: { visible: false } },
    goal: { dialog: { visible: false } },
    settings: { dialog: { visible: false, section: DEFAULT_SETTINGS_SECTION } },
    commander: { dialog: { visible: false } },
    tabs: {
      items: [INITIAL_APP_TAB] as ViewportTabState[],
      current: "aos" as string,
    },
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .addAction("toggle", (ctx) => (path: string, visible?: boolean) => {
    const keys = path.split(".");
    if (keys.length === 0) return;

    ctx.state.set((state) => {
      const newState = { ...state };
      let current: any = newState;
      for (let i = 0; i < keys.length - 1; i++) {
        if (!current[keys[i]]) current[keys[i]] = {};
        current = current[keys[i]];
      }
      const lastKey = keys[keys.length - 1];
      current[lastKey] = visible ?? !current[lastKey];
      return newState;
    });
  })
  .addAction("setTaskDialogProject", (ctx) => (project?: string) => {
    ctx.state.set((state) => ({
      tasks: { ...state.tasks, dialog: { ...state.tasks.dialog, project } },
    }));
  })
  .addAction(
    "updateInAppMetadata",
    (ctx) =>
      (metadata: {
        title: string;
        description: string;
        hasPageSidebar?: boolean;
        hasPageDetails?: boolean;
      }) => {
        ctx.state.set((state) => ({
          page: {
            sidebar: {
              ...state.page.sidebar,
              enabled: metadata.hasPageSidebar ?? state.page.sidebar.enabled,
            },
            details: {
              ...state.page.details,
              enabled: metadata.hasPageDetails ?? state.page.details.enabled,
            },
          },
          tabs: {
            ...state.tabs,
            items: state.tabs.items.map((tab) =>
              tab.id === "aos"
                ? {
                    ...tab,
                    title: metadata.title,
                    metadata: {
                      ...tab.metadata,
                      hasPageSidebar: metadata.hasPageSidebar,
                      hasPageDetails: metadata.hasPageDetails,
                    },
                  }
                : tab,
            ),
          },
        }));
      },
  )
  .addAction("fullscreen", (ctx) => (focus: "chat" | "page" | undefined) => {
    if (!focus) {
      ctx.state.set((state) => ({
        fullscreen: undefined,
        layout: {
          ...state.layout,
          sidebar: { ...state.layout.sidebar, visible: true },
        },
        page: {
          ...state.page,
          sidebar: { ...state.page.sidebar, visible: true },
          details: { ...state.page.details, visible: false },
        },
        agent: {
          ...state.agent,
          history: { ...state.agent.history, visible: false },
          panel: { ...state.agent.panel, visible: true },
        },
        inbox: {
          ...state.inbox,
          panel: { ...state.inbox.panel, visible: false },
        },
      }));
      return;
    }

    if (focus === "chat")
      ctx.state.set((state) => ({
        fullscreen: "chat",
        layout: {
          ...state.layout,
          sidebar: { ...state.layout.sidebar, visible: false },
        },
        page: {
          ...state.page,
          sidebar: { ...state.page.sidebar, visible: false },
          details: { ...state.page.details, visible: false },
        },
        agent: {
          ...state.agent,
          history: { ...state.agent.history, visible: false },
          panel: { ...state.agent.panel, visible: true },
        },
        inbox: {
          ...state.inbox,
          panel: { ...state.inbox.panel, visible: false },
        },
      }));
    if (focus === "page")
      ctx.state.set((state) => ({
        fullscreen: "page",
        layout: {
          ...state.layout,
          sidebar: { ...state.layout.sidebar, visible: false },
        },
        page: {
          ...state.page,
          sidebar: { ...state.page.sidebar, visible: true },
          details: { ...state.page.details, visible: true },
        },
        agent: {
          ...state.agent,
          history: { ...state.agent.history, visible: false },
          panel: { ...state.agent.panel, visible: true },
        },
        inbox: {
          ...state.inbox,
          panel: { ...state.inbox.panel, visible: false },
        },
      }));
  })
  .addAction("openSettings", (ctx) => (section?: SettingsSectionId, selection?: SettingsSelection) => {
    const nextSection =
      section ??
      ctx.state.get().settings.dialog.section ??
      DEFAULT_SETTINGS_SECTION;

    ctx.state.set((state) => ({
      layout: {
        ...state.layout,
        sidebar: { ...state.layout.sidebar, menu: "settings" },
      },
      settings: {
        dialog: {
          visible: false,
          section: nextSection,
        },
      },
    }));

    void navigateToSettingsSection(nextSection, selection);
  })
  .addAction("closeSettings", (ctx) => () => {
    ctx.state.set((state) => ({
      layout: {
        ...state.layout,
        sidebar: { ...state.layout.sidebar, menu: "main" },
      },
      settings: {
        ...state.settings,
        dialog: { ...state.settings.dialog, visible: false },
      },
    }));

    void import("@/app/router").then(({ router }) => {
      if (router.state.location.pathname.startsWith("/settings")) {
        void router.navigate({ to: "/" });
      }
    });
  })
  .addAction("setSettingsSection", (ctx) => (section: SettingsSectionId) => {
    ctx.state.set((state) => ({
      layout: {
        ...state.layout,
        sidebar: { ...state.layout.sidebar, menu: "settings" },
      },
      settings: {
        ...state.settings,
        dialog: { ...state.settings.dialog, section },
      },
    }));

    void navigateToSettingsSection(section);
  })
  .addAction("setActiveTab", (ctx) => (tabId: string) => {
    ctx.state.set({ tabs: { current: tabId } });
  })
  .addAction("createTab", (ctx) => (input?: Partial<ViewportTabState>) => {
    const id = input?.id || generateId();

    const newTab: ViewportTabState = {
      id,
      type: input?.type ?? "browser",
      title: "New tab",
      url: "https://duckduckgo.com/",
      status: "idle",
      canGoBack: false,
      canGoForward: false,
      closable: true,
      ...input,
    };

    ctx.state.set((state) => ({
      tabs: {
        items: [...state.tabs.items, newTab],
        current: id,
      },
    }));

    return id;
  })
  .addAction("closeTab", (ctx) => (tabId: string) => {
    const state = ctx.state.get();
    if (tabId === "aos") return;

    const tabIndex = state.tabs.items.findIndex((t) => t.id === tabId);
    if (tabIndex === -1) return;

    const nextTabs = state.tabs.items.filter((t) => t.id !== tabId);
    let nextActiveId = state.tabs.current;

    if (state.tabs.current === tabId) {
      nextActiveId = state.tabs.items[tabIndex - 1]?.id || "aos";
    }

    ctx.state.set({
      tabs: {
        items: nextTabs,
        current: nextActiveId,
      },
    });
  })
  .addAction("reorderTabs", (ctx) => (activeId: string, overId: string) => {
    if (activeId === overId) return;
    if (activeId === "aos" || overId === "aos") return;

    const state = ctx.state.get();
    const anchorIndex = state.tabs.items.findIndex((tab) => tab.id === "aos");
    if (anchorIndex === -1) return;

    const anchor = state.tabs.items[anchorIndex];
    const movable = state.tabs.items.filter((tab) => tab.id !== "aos");

    const oldIndex = movable.findIndex((tab) => tab.id === activeId);
    const newIndex = movable.findIndex((tab) => tab.id === overId);
    if (oldIndex === -1 || newIndex === -1) return;

    ctx.state.set((current) => ({
      tabs: {
        ...current.tabs,
        items: [anchor, ...arrayMove(movable, oldIndex, newIndex)],
      },
    }));
  })
  .addAction(
    "updateTab",
    (ctx) => (tabId: string, patch: Partial<ViewportTabState>) => {
      ctx.state.set((state) => ({
        tabs: {
          ...state.tabs,
          items: state.tabs.items.map((tab) =>
            tab.id === tabId ? { ...tab, ...patch } : tab,
          ),
        },
      }));
    },
  )
  .addAction("setSidebarMenu", (ctx) => (menu: WorkspaceSidebarMenu) => {
    ctx.state.set((state) => ({
      layout: { ...state.layout, sidebar: { ...state.layout.sidebar, menu } },
    }));
  })
  .addAction("setSidebarWidth", (ctx) => (width: number) => {
    ctx.state.set((state) => ({
      layout: { ...state.layout, sidebar: { ...state.layout.sidebar, width } },
    }));
  })
  .addAction("toggleCommander", (ctx) => (open?: boolean) => {
    ctx.state.set((state) => ({
      commander: {
        dialog: { visible: open ?? !state.commander.dialog.visible },
      },
    }));
  })
  .addAction("setCommanderOpen", (ctx) => (open: boolean) => {
    ctx.state.set({ commander: { dialog: { visible: open } } });
  })
  .withPersistence({
    enabled: true,
    storage: "localstorage",
    pick: (state) => ({
      tabs: state.tabs,
      layout: { sidebar: { width: state.layout.sidebar.width } },
    }),
  })
  .withBroadcast({ enabled: true })
  .build();
