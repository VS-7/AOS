import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

type Tab = { id: string; type: string; title: string; metadata: Record<string, string> };

const viewport = vi.hoisted(() => ({
  state: { tabs: { items: [] as Tab[] } },
  actions: {
    updateTab: vi.fn((id: string, patch: { title: string }) => {
      const tab = viewport.state.tabs.items.find((item) => item.id === id);
      if (tab) tab.title = patch.title;
    }),
    closeTab: vi.fn(),
  },
  useState: (select: (s: unknown) => unknown) => select(viewport.state),
}));
const liveChat = vi.hoisted(() => ({ chat: undefined as unknown }));

vi.mock("@/app/aos", () => ({
  aos: {
    stores: {
      viewport,
      agent: { useState: () => ({ items: [{ id: "luara", name: "Luara" }] }) },
      workspace: { useState: (select: (s: unknown) => unknown) => select({ directory: { users: [] } }) },
      auth: { useState: (select: (s: unknown) => unknown) => select({ user: { id: "me", name: "Vitor" } }) },
    },
  },
}));
// The avatar library does not load under jsdom, and the header's icon is not what is tested.
vi.mock("@/components/ui/avatar", () => ({ AvatarAgentFallback: () => null }));
vi.mock("@/features/chat/presentation/hooks/use-chat", () => ({
  useChat: () => ({
    chat: liveChat.chat,
    error: undefined,
    isRefreshing: false,
    persistedMessageIds: new Set(),
    refresh: vi.fn(),
    replaceAndRefresh: vi.fn(),
    replaceMessage: vi.fn(),
    removeMessage: vi.fn(),
    appendMessage: vi.fn(),
  }),
}));
vi.mock("../hooks/use-chat-actions", () => ({
  useChatActions: () => ({ rename: vi.fn(), clear: vi.fn(), remove: vi.fn(), isRenaming: false }),
}));
vi.mock("@/features/chat/presentation/pages/($id)/components/chat-message-list.component", () => ({
  ChatMessageList: () => null,
}));
vi.mock("@/features/chat/presentation/pages/($id)/components/chat-composer.component", () => ({
  ChatComposer: () => null,
}));
vi.mock(
  "@/features/workspace/presentation/components/sidebar/components/menus/main/groups/chat/components/chat-row-kind-icon",
  () => ({ ChatRowKindIcon: () => null }),
);

import { ChatContent } from "./chat-content";

beforeAll(() => {
  // Radix measures what it positions; jsdom has none of these.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never;
  Element.prototype.scrollIntoView ??= () => {};
  Element.prototype.hasPointerCapture ??= () => false;
});

const agentDm = {
  id: "c-luara",
  kind: "dm",
  title: "Luara renomeada",
  participants: [{ type: "agent", id: "luara" }],
};
const channel = { id: "c-geral", kind: "channel", title: "geral", participants: [] };

beforeEach(() => {
  viewport.actions.updateTab.mockClear();
});
afterEach(cleanup);

function openActions() {
  fireEvent.keyDown(screen.getByRole("button", { name: "Conversation actions" }), { key: "Enter" });
  return screen.getAllByRole("menuitem").map((item) => item.textContent);
}

describe("ChatContent", () => {
  // A DM with an agent is named after the agent in its header, its tab and
  // the Team list. Renaming it said "Conversation renamed." and stored the
  // title, and nothing on screen changed.
  it("does not offer to rename a DM with an agent", () => {
    liveChat.chat = agentDm;
    viewport.state.tabs.items = [{ id: "t1", type: "chat", title: "Luara", metadata: { chatId: agentDm.id } }];
    render(<ChatContent chatId={agentDm.id} />);

    expect(screen.getByRole("heading", { name: "Luara" })).toBeTruthy();
    const items = openActions();
    expect(items).not.toContain("Rename");
    expect(items).toContain("Clear messages");
  });

  it("still offers to rename a channel", () => {
    liveChat.chat = channel;
    viewport.state.tabs.items = [{ id: "t2", type: "chat", title: "geral", metadata: { chatId: channel.id } }];
    render(<ChatContent chatId={channel.id} />);

    expect(openActions()).toContain("Rename");
  });

  // Opening the conversation again from search passes its stored title,
  // which replaced the tab's name while the header kept the agent's — and
  // the header's title had not changed, so nothing put the tab back.
  it("puts the tab's title back when something else renames the tab", () => {
    liveChat.chat = agentDm;
    viewport.state.tabs.items = [{ id: "t1", type: "chat", title: "Luara", metadata: { chatId: agentDm.id } }];
    const { rerender } = render(<ChatContent chatId={agentDm.id} />);

    viewport.state.tabs.items[0].title = "Luara renomeada";
    rerender(<ChatContent chatId={agentDm.id} />);

    expect(viewport.state.tabs.items[0].title).toBe("Luara");
  });
});
