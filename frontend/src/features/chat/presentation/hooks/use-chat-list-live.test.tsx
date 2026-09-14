import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { cleanup, renderHook } from "@testing-library/react";

const refresh = vi.hoisted(() => vi.fn());
vi.mock("@/app/aos", () => ({ aos: { stores: { chat: { actions: { refresh } } } } }));

import { deliver } from "@/lib/realtime";
import { useChatListLive } from "./use-chat-list-live";

const qc = { invalidateQueries: vi.fn(), setQueryData: vi.fn() } as never;

beforeEach(() => {
  vi.useFakeTimers();
  refresh.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("useChatListLive", () => {
  // The sidebar's roster is a preloaded store, which no query invalidation
  // reaches. A channel created here, a DM opened from Team, an agent's own
  // chat: none appeared until the window was reloaded.
  it("refreshes the roster once for a burst of chat writes", () => {
    renderHook(() => useChatListLive());

    deliver(qc, { type: "collection.changed", data: { collection: "chats", op: "create" } });
    deliver(qc, { type: "collection.changed", data: { collection: "chats", op: "update" } });
    deliver(qc, { type: "chat.done", data: { chat: "c-1", agent: "luara" } });
    expect(refresh).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1_000);
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it("ignores writes to other collections", () => {
    renderHook(() => useChatListLive());
    deliver(qc, { type: "collection.changed", data: { collection: "tasks", op: "update" } });
    vi.advanceTimersByTime(1_000);
    expect(refresh).not.toHaveBeenCalled();
  });

  it("stops listening when the sidebar unmounts", () => {
    const { unmount } = renderHook(() => useChatListLive());
    deliver(qc, { type: "collection.changed", data: { collection: "chats" } });
    unmount();
    vi.advanceTimersByTime(1_000);
    deliver(qc, { type: "collection.changed", data: { collection: "chats" } });
    vi.advanceTimersByTime(1_000);
    expect(refresh).not.toHaveBeenCalled();
  });
});
