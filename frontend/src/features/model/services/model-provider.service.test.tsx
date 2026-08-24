import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";

/**
 * The merge is the part that decides what a person sees in the model picker,
 * and it is the part a browser check cannot pin down: the row renders a count,
 * not the reasoning behind it. These assert the three states directly —
 * discovered, not connected, and connected but unreachable — because the third
 * one is the one that used to be indistinguishable from the first.
 */

const state = vi.hoisted(() => ({
  connected: [] as { id: string; key?: string }[],
  answer: null as unknown,
}));

vi.mock("@/app/aos", () => ({
  aos: {
    useContext: () => ({ config: { agents: { providers: state.connected } } }),
    client: {
      model: {
        list: {
          useQuery: () => ({ data: state.answer, isPending: false }),
        },
      },
    },
  },
}));

// Imported after the mock, so the module graph sees the fake `aos`.
const { useModelProviders } = await import("./model-provider.service");

function providerNamed(id: string) {
  const { result } = renderHook(() => useModelProviders());
  const found = result.current.find((p) => p.id === id);
  if (!found) throw new Error(`no ${id} in the catalog`);
  return found;
}

describe("useModelProviders", () => {
  it("shows what the provider itself answered, not what this build has written down", () => {
    state.connected = [{ id: "codex", key: "" }];
    state.answer = {
      providers: [
        {
          id: "codex",
          models: [
            { id: "gpt-5.6-sol", name: "GPT-5.6-Sol" },
            { id: "gpt-5.4", name: "GPT-5.4" },
          ],
        },
      ],
      total: 2,
    };

    const codex = providerNamed("codex");
    expect(codex.configured).toBe(true);
    expect(codex.modelsDiscovered).toBe(true);
    expect(codex.models.map((m) => m.id)).toEqual(["gpt-5.6-sol", "gpt-5.4"]);
    // The provider's order is the provider's ranking, and the first entry is
    // what a picker offers — so it must survive the merge unsorted.
    expect(codex.models[0].name).toBe("GPT-5.6-Sol");
    // Nothing here claims a capability: no adapter in this build does
    // anything past text and tool calls, so the realtime/voice/image/video
    // slots stay honestly empty rather than offering a turn that would fail.
    expect(codex.models[0].capabilities).toBeUndefined();
  });

  it("falls back to the static catalog for a provider with no credential to ask with", () => {
    state.connected = [{ id: "codex", key: "" }];
    state.answer = { providers: [{ id: "codex", models: [{ id: "gpt-5.4", name: "GPT-5.4" }] }], total: 1 };

    const anthropic = providerNamed("anthropic");
    expect(anthropic.configured).toBe(false);
    expect(anthropic.modelsDiscovered).toBeFalsy();
    expect(anthropic.models.length).toBeGreaterThan(0);
    expect(anthropic.modelsError).toBeUndefined();
  });

  it("keeps the reason when a connected provider could not be asked", () => {
    state.connected = [{ id: "codex", key: "" }];
    state.answer = {
      providers: [{ id: "codex", models: [], error: "the codex provider answered 401" }],
      total: 0,
    };

    const codex = providerNamed("codex");
    expect(codex.configured).toBe(true);
    // Connected, so the picker still has to offer something — but the screen
    // must be able to say that these names are the fallback, not this
    // account's. Reporting the failure as an empty list is the silence this
    // whole change exists to end.
    expect(codex.modelsDiscovered).toBe(false);
    expect(codex.models.length).toBeGreaterThan(0);
    expect(codex.modelsError).toContain("401");
  });
});
