import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * Connecting and disconnecting a provider are configuration writes, and the
 * defects were in what those writes left behind: a Default slot still naming
 * a provider that had just been disconnected (so the screen said "Select
 * model", connecting another provider did not fill it, and the next chat went
 * out to the old provider without a key), and a Default seeded from this
 * build's static list after the provider had refused the key.
 */

const daemon = vi.hoisted(() => ({
  config: { agents: { providers: [] as { id: string; key?: string }[], models: {} as Record<string, unknown> } },
  catalogue: {} as Record<string, { models?: { id: string }[]; error?: string; cta?: { label?: string }[] }>,
  writes: [] as unknown[],
}));

vi.mock("@/app/aos", () => ({
  aos: {
    client: {
      config: {
        get: { query: async () => ({ data: structuredClone(daemon.config), error: null }) },
        update: {
          mutate: async ({ body }: { body: { agents: Record<string, unknown> } }) => {
            daemon.writes.push(structuredClone(body));
            daemon.config.agents = { ...daemon.config.agents, ...structuredClone(body.agents) } as never;
            return { data: daemon.config, error: null };
          },
        },
      },
      model: {
        list: {
          query: async ({ query }: { query: { provider: string } }) => ({
            data: { providers: [{ id: query.provider, models: [], ...daemon.catalogue[query.provider] }] },
            error: null,
          }),
        },
      },
    },
    stores: { config: { actions: { refresh: async () => undefined } } },
  },
}));

const { setModelProviderKey, disconnectModelProvider } = await import("./model-provider.service");

beforeEach(() => {
  daemon.config = { agents: { providers: [], models: {} } };
  daemon.catalogue = {};
  daemon.writes = [];
});

describe("disconnectModelProvider", () => {
  it("clears every slot that pointed at the provider, in the same write", async () => {
    daemon.config.agents.providers = [{ id: "anthropic", key: "sk-ant" }, { id: "openai", key: "sk" }];
    daemon.config.agents.models = {
      default: { provider: "anthropic", model: "claude-opus-5", reasoning: "high" },
      subconscious: { provider: "openai", model: "gpt-5.1", reasoning: "medium" },
    };

    await disconnectModelProvider("anthropic");

    expect(daemon.writes).toHaveLength(1);
    expect(daemon.config.agents.providers).toEqual([{ id: "openai", key: "sk" }]);
    expect(daemon.config.agents.models).toEqual({
      subconscious: { provider: "openai", model: "gpt-5.1", reasoning: "medium" },
    });
  });
});

describe("setModelProviderKey", () => {
  it("fills a Default slot left pointing at a provider that is no longer connected", async () => {
    daemon.config.agents.models = { default: { provider: "anthropic", model: "claude-opus-5", reasoning: "high" } };
    daemon.catalogue.openai = { models: [{ id: "gpt-5.1" }] };

    const outcome = await setModelProviderKey("openai", "sk-good");

    expect(outcome.error).toBeUndefined();
    expect(daemon.config.agents.models.default).toEqual({ provider: "openai", model: "gpt-5.1", reasoning: "high" });
  });

  it("does not seed Default from the static list when the provider refused the key", async () => {
    daemon.catalogue.anthropic = {
      error: "AOS_PROVIDER_REFUSED: the anthropic provider answered 401: API key is invalid.",
      cta: [{ label: "check the key" }],
    };

    const outcome = await setModelProviderKey("anthropic", "sk-ant-invalid");

    expect(outcome.error).toContain("API key is invalid.");
    expect(outcome.actions).toEqual(["check the key"]);
    expect(daemon.config.agents.providers).toEqual([{ id: "anthropic", key: "sk-ant-invalid" }]);
    expect(daemon.config.agents.models).toEqual({});
  });

  it("never replaces a Default that belongs to a connected provider", async () => {
    daemon.config.agents.providers = [{ id: "openai", key: "sk" }];
    daemon.config.agents.models = { default: { provider: "openai", model: "o3", reasoning: "medium" } };
    daemon.catalogue.anthropic = { models: [{ id: "claude-opus-5" }] };

    await setModelProviderKey("anthropic", "sk-ant");

    expect(daemon.config.agents.models.default).toEqual({ provider: "openai", model: "o3", reasoning: "medium" });
  });
});
