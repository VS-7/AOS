import { describe, it, expect, vi, beforeEach } from "vitest";

const list = vi.hoisted(() => vi.fn());
const create = vi.hoisted(() => vi.fn());
const viewport = vi.hoisted(() => ({
  state: { tabs: { items: [] as Array<Record<string, unknown>> } },
  actions: {
    createTab: vi.fn(() => "tab-1"),
    setActiveTab: vi.fn(),
    updateTab: vi.fn(),
    closeTab: vi.fn(),
  },
}));
vi.mock("@/app/aos", () => ({
  aos: {
    client: { chat: { list: { query: list }, create: { mutate: create } } },
    stores: { viewport, auth: { state: { user: { id: "me" } } } },
  },
}));

import { openUserDmTab, syncChatTabTitle, teamPeople } from "./open-chat-tab.helper";

beforeEach(() => {
  list.mockReset();
  create.mockReset();
  viewport.state.tabs.items = [];
});

describe("openUserDmTab", () => {
  // Naming the signed-in person as the peer and then adding them again as
  // themselves stored a conversation with the same user twice.
  it("names a person once even when the peer is the signed-in user", async () => {
    list.mockResolvedValue({ data: { chats: [] }, error: undefined });
    create.mockResolvedValue({ data: { chat: { id: "c-1", title: "Vitor" } }, error: undefined });

    await openUserDmTab({ userId: "me", title: "Vitor" });

    const participants = create.mock.calls[0][0].body.participants;
    expect(participants).toEqual([{ type: "user", id: "me" }]);
  });

  it("names both people for a DM with somebody else", async () => {
    list.mockResolvedValue({ data: { chats: [] }, error: undefined });
    create.mockResolvedValue({ data: { chat: { id: "c-2", title: "Ana" } }, error: undefined });

    await openUserDmTab({ userId: "ana", title: "Ana" });

    expect(create.mock.calls[0][0].body.participants).toEqual([
      { type: "user", id: "ana" },
      { type: "user", id: "me" },
    ]);
  });
});

describe("teamPeople", () => {
  // The workspace directory lists every account, the viewer included — the
  // task assignee pickers need that. The Team tab is a list of people to
  // start a conversation with, and the viewer is not one of them.
  it("leaves the signed-in person out", () => {
    const users = [{ id: "me", name: "Vitor" }, { id: "ana", name: "Ana" }];
    expect(teamPeople(users, "me").map((u) => u.id)).toEqual(["ana"]);
    expect(teamPeople(users, undefined).map((u) => u.id)).toEqual(["me", "ana"]);
  });
});

describe("syncChatTabTitle", () => {
  // A deep link or a restored tab only knows the id; nothing named it after
  // the conversation once the conversation loaded.
  it("renames the chat's tab once, and leaves other tabs alone", () => {
    viewport.state.tabs.items = [
      { id: "t-1", type: "chat", title: "42ea4b04-1569-45cc", metadata: { chatId: "42ea4b04-1569-45cc" } },
      { id: "t-2", type: "chat", title: "Other", metadata: { chatId: "c-9" } },
    ];
    viewport.actions.updateTab.mockClear();

    syncChatTabTitle("42ea4b04-1569-45cc", "Luara");
    expect(viewport.actions.updateTab).toHaveBeenCalledWith("t-1", { title: "Luara" });

    viewport.actions.updateTab.mockClear();
    viewport.state.tabs.items[0].title = "Luara";
    syncChatTabTitle("42ea4b04-1569-45cc", "Luara");
    syncChatTabTitle("missing", "Nobody");
    expect(viewport.actions.updateTab).not.toHaveBeenCalled();
  });
});
