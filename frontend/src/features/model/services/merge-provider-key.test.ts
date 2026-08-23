import { describe, expect, it } from "vitest";
import { mergeProviderKey, removeProvider } from "./merge-provider-key";

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

  // The regression this file exists to pin down. `codex`/`gemini-cli` are
  // connected with no key of their own — the credential is another tool's
  // file on this machine — so a blank key has to persist an entry. It used
  // to delete one instead, which made connecting either of them a write of
  // `agents.providers: []`: HTTP 200, "connected" toast, nothing saved.
  it("keeps an entry for a blank key, which is how an oauth-file provider connects", () => {
    const existing = [{ id: "anthropic", key: "sk-ant-real-secret" }];
    expect(mergeProviderKey(existing, "codex", "")).toEqual([
      { id: "anthropic", key: "sk-ant-real-secret" },
      { id: "codex", key: "" },
    ]);
  });

  it("trims whitespace around a key, and treats an all-whitespace one as blank", () => {
    expect(mergeProviderKey([], "openai", "  sk-y  ")).toEqual([
      { id: "openai", key: "sk-y" },
    ]);
    expect(mergeProviderKey([], "codex", "   ")).toEqual([
      { id: "codex", key: "" },
    ]);
  });
});

describe("removeProvider", () => {
  it("drops only the named provider", () => {
    const existing = [
      { id: "anthropic", key: "sk-ant-real-secret" },
      { id: "openai", key: "sk-openai" },
    ];
    expect(removeProvider(existing, "openai")).toEqual([
      { id: "anthropic", key: "sk-ant-real-secret" },
    ]);
  });

  it("removes a keyless entry too — the oauth-file case mergeProviderKey now keeps", () => {
    expect(removeProvider([{ id: "codex", key: "" }], "codex")).toEqual([]);
  });

  it("is a no-op when nothing matches", () => {
    const existing = [{ id: "anthropic", key: "sk-ant" }];
    expect(removeProvider(existing, "openai")).toEqual(existing);
  });
});
