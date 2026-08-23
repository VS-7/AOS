import { describe, expect, it } from "vitest";
import { mergeProviderKey } from "./merge-provider-key";

describe("mergeProviderKey", () => {
  it("adds a new provider to an empty list", () => {
    expect(mergeProviderKey([], "openai", "sk-123")).toEqual([
      { id: "openai", key: "sk-123" },
    ]);
  });

  it("leaves every other provider's real key untouched — the property setModelProviderKey exists to protect", () => {
    // This is the failure mode a naive redacted-read-then-write-back would
    // cause: connecting a second provider would silently overwrite the
    // first one's real key with its own fingerprint string.
    const existing = [
      { id: "anthropic", key: "sk-ant-real-secret" },
      { id: "google", key: "AIza-real-secret" },
    ];
    expect(mergeProviderKey(existing, "openai", "sk-openai-new")).toEqual([
      { id: "anthropic", key: "sk-ant-real-secret" },
      { id: "google", key: "AIza-real-secret" },
      { id: "openai", key: "sk-openai-new" },
    ]);
  });

  it("replaces an existing provider's key rather than duplicating the entry", () => {
    const existing = [{ id: "openai", key: "sk-old" }];
    expect(mergeProviderKey(existing, "openai", "sk-new")).toEqual([
      { id: "openai", key: "sk-new" },
    ]);
  });

  it("removes the entry entirely on an empty key, rather than storing a blank one", () => {
    const existing = [
      { id: "anthropic", key: "sk-ant-real-secret" },
      { id: "openai", key: "sk-openai" },
    ];
    expect(mergeProviderKey(existing, "openai", "")).toEqual([
      { id: "anthropic", key: "sk-ant-real-secret" },
    ]);
  });

  it("trims whitespace before deciding whether a key is present", () => {
    expect(mergeProviderKey([{ id: "openai", key: "sk-x" }], "openai", "   ")).toEqual(
      [],
    );
    expect(mergeProviderKey([], "openai", "  sk-y  ")).toEqual([
      { id: "openai", key: "sk-y" },
    ]);
  });
});
