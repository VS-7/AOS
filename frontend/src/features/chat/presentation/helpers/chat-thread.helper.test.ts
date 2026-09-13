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

describe("ChatThreadHelper.formatReactionTooltip", () => {
  const people = {
    selfUserId: "325e3e10",
    usersById: new Map([["ana-id", { id: "ana-id", name: "Ana", username: "ana" }]]),
    agents: [{ id: "luara", name: "Luara" }] as never,
  };

  // The daemon records who reacted by id. The tooltip compared those ids
  // with the viewer's display name, never matched, and printed
  // "325e3e10-64d1-… reacted with 👍".
  it("names the viewer as You, and everyone else by name", () => {
    expect(ChatThreadHelper.formatReactionTooltip({ ...people, actors: ["325e3e10"], emoji: "👍" })).toBe("You reacted with 👍");
    expect(ChatThreadHelper.formatReactionTooltip({ ...people, actors: ["325e3e10", "ana-id"], emoji: "👍" })).toBe("You and Ana reacted with 👍");
    expect(
      ChatThreadHelper.formatReactionTooltip({ ...people, actors: ["luara", "ana-id", "x", "y"], emoji: "❤️" }),
    ).toBe("Luara, Ana and 2 others reacted with ❤️");
    expect(ChatThreadHelper.formatReactionTooltip({ ...people, actors: ["luara", "ana-id", "x"], emoji: "❤️" })).toBe(
      "Luara, Ana and 1 other reacted with ❤️",
    );
  });

  it("falls back to the id for somebody it cannot name", () => {
    expect(ChatThreadHelper.formatReactionTooltip({ ...people, actors: ["gone-user"], emoji: "👍" })).toBe("gone-user reacted with 👍");
  });
});

describe("ChatThreadHelper dates", () => {
  // Day dividers and times were formatted as en-US whatever the interface
  // language: "Sat, Sep 12" and "5:49 PM" in a Portuguese window.
  it("formats in the interface's language", () => {
    const at = new Date(2026, 8, 12, 17, 49);
    expect(ChatThreadHelper.formatMessageTime(at, "pt-BR")).toBe("17:49");
    expect(ChatThreadHelper.formatMessageDay(at, "pt-BR")).toMatch(/12/);
    expect(ChatThreadHelper.formatMessageDay(at, "pt-BR")).toMatch(/set/i);
    expect(ChatThreadHelper.formatMessageTime(at, "en")).toMatch(/5:49\s?PM/);
  });
});
