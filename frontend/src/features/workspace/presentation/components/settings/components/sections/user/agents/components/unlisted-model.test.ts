import { describe, it, expect } from "vitest";
import type { ModelProvider } from "@/features/model/interfaces/model.interfaces";
import { unlistedModel } from "./unlisted-model";

const codex = (over: Partial<ModelProvider> = {}): ModelProvider => ({
  id: "codex",
  name: "Codex",
  description: "",
  logo: { light: "", dark: "" },
  configured: true,
  default: false,
  auth: { mode: "oauth-file", connectionType: "local", label: "", placeholder: "", description: "", required: false },
  models: [
    { id: "gpt-5.5", name: "GPT-5.5", enabled: true },
    { id: "gpt-5.6-terra", name: "GPT-5.6 Terra", enabled: true },
  ],
  modelsDiscovered: true,
  ...over,
});

// The Default slot still named codex/gpt-5.4-mini after the provider stopped
// offering it, and the slot showed it as if nothing were wrong. Every agent
// without a model of its own failed every turn, and Settings — the one place
// that could have said so before a message was sent — said nothing.
describe("unlistedModel", () => {
  it("names a saved model the provider no longer lists", () => {
    expect(unlistedModel([codex()], { provider: "codex", model: "gpt-5.4-mini" })).toBe("gpt-5.4-mini");
  });

  it("is quiet for a model the provider lists", () => {
    expect(unlistedModel([codex()], { provider: "codex", model: "gpt-5.5" })).toBeNull();
  });

  // Only the provider's own answer is evidence. The static fallback is what
  // was true when somebody typed it, and a catalogue that failed to load says
  // nothing about any model.
  it("does not judge against a list the provider never answered", () => {
    expect(unlistedModel([codex({ modelsDiscovered: false })], { provider: "codex", model: "gpt-5.4-mini" })).toBeNull();
    expect(unlistedModel([codex({ modelsError: "AOS_OAUTH_REFRESH_FAILED" })], { provider: "codex", model: "gpt-5.4-mini" })).toBeNull();
    expect(unlistedModel([codex({ configured: false })], { provider: "codex", model: "gpt-5.4-mini" })).toBeNull();
  });

  it("is quiet for an empty slot", () => {
    expect(unlistedModel([codex()], { provider: "", model: "" })).toBeNull();
  });
});
