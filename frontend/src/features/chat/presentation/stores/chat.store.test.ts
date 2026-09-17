import { describe, it, expect, vi, beforeEach } from "vitest";

const query = vi.hoisted(() => vi.fn());
vi.mock("@/lib/aos-facade", () => ({ api: { chat: { list: { query } } } }));

import { ChatStore } from "./chat.store";
import { ChatKindHelper } from "../../services/chat/chat-kind.helper";

const chat = (id: string) => ({ id, title: id, kind: "channel", participants: [], messages: [] });

beforeEach(() => {
  query.mockReset();
});

describe("ChatStore.refresh", () => {
  it("replaces the list with what the daemon answered", async () => {
    query.mockResolvedValueOnce({ data: { chats: [chat("a"), chat("b")] }, error: undefined });
    await ChatStore.actions.refresh();
    expect(ChatStore.state.items.map((c) => c.id)).toEqual(["a", "b"]);
  });

  // A failed read is not an empty list. Storing `undefined` here crashed the
  // whole window: every sidebar tab calls `.filter` on it during render, and
  // the error boundary took the application with it.
  it("keeps what it had when the read fails", async () => {
    query.mockResolvedValueOnce({ data: { chats: [chat("kept")] }, error: undefined });
    await ChatStore.actions.refresh();

    query.mockResolvedValueOnce({ data: undefined, error: { code: "AOS_CHAT_READ_FAILED", message: "no" } });
    await ChatStore.actions.refresh();

    expect(ChatStore.state.items.map((c) => c.id)).toEqual(["kept"]);
  });

  it("reads an answer with no chats field as empty, not as missing", async () => {
    query.mockResolvedValueOnce({ data: {}, error: undefined });
    await ChatStore.actions.refresh();
    expect(ChatStore.state.items).toEqual([]);
  });
});

describe("ChatKindHelper on a list that is not there", () => {
  it("counts nothing instead of throwing", () => {
    expect(ChatKindHelper.filterByKind(undefined as never, "channel", new Set())).toEqual([]);
    expect(ChatKindHelper.countProcessingByKind(null as never, "task", {}, new Set())).toBe(0);
  });
});
