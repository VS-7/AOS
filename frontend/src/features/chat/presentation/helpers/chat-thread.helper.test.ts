import { describe, it, expect } from "vitest";
import { ChatThreadHelper } from "./chat-thread.helper";

const message = (id: string, role: "user" | "assistant", author = "me") =>
  ({ id, role, parts: [], metadata: { type: role === "user" ? "user" : "agent", data: { id: author } } }) as never;

describe("ChatThreadHelper.shouldFollowNewest", () => {
  // Sending while scrolled up added the message below the fold, and nothing
  // on screen changed: the list only followed new items while already at the
  // bottom.
  it("follows the person's own message that has not been confirmed yet", () => {
    expect(
      ChatThreadHelper.shouldFollowNewest({
        atBottom: false,
        newest: message("local-1", "user"),
        persistedIds: new Set(["m-1"]),
        selfUserId: "me",
      }),
    ).toBe(true);
  });

  it("does not yank the reader down for somebody else's message", () => {
    expect(
      ChatThreadHelper.shouldFollowNewest({
        atBottom: false,
        newest: message("m-2", "assistant", "luara"),
        persistedIds: new Set(),
        selfUserId: "me",
      }),
    ).toBe(false);
    expect(
      ChatThreadHelper.shouldFollowNewest({
        atBottom: false,
        newest: message("m-3", "user", "ana"),
        persistedIds: new Set(),
        selfUserId: "me",
      }),
    ).toBe(false);
  });

  it("keeps following anything while already at the bottom", () => {
    expect(
      ChatThreadHelper.shouldFollowNewest({
        atBottom: true,
        newest: message("m-2", "assistant", "luara"),
        persistedIds: new Set(["m-2"]),
        selfUserId: "me",
      }),
    ).toBe(true);
  });
});
