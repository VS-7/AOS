import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The roster is what the agents screen searches, groups and follows. It kept
 * six fields of the record, so search by provider or model never matched and
 * grouping by skill always said "General" — and a failed read replaced it
 * with nothing, which the editor reads as every agent having been deleted.
 */

const list = vi.hoisted(() => ({ answer: null as unknown }));

vi.mock("@/lib/aos-facade", () => ({
  api: {
    agent: { list: { query: async () => list.answer } },
    chat: { list: { query: async () => ({ data: { active: {} }, error: null }) } },
  },
}));

const { AgentStore } = await import("./agent.store");

const builder = {
  id: "api-builder",
  name: "API Builder",
  provider: "codex",
  model: "gpt-5.6-terra",
  skill: "backend",
  leader: "luara",
  reasoning: "medium",
  sandbox: { permissions: ["read"] },
  updatedAt: "2026-08-30T13:47:06Z",
  orchestrator: false,
};

beforeEach(() => {
  list.answer = { data: { agents: [builder], total: 1 }, error: null };
});

describe("AgentStore", () => {
  it("keeps every field the roster answers with", async () => {
    await AgentStore.init();
    await AgentStore.actions.refresh();

    expect(AgentStore.state.items).toEqual([builder]);
  });

  it("keeps the roster it has when a refresh cannot read a new one", async () => {
    await AgentStore.init();
    await AgentStore.actions.refresh();

    list.answer = { data: undefined, error: { code: "AOS_TRANSPORT", message: "down" } };
    vi.spyOn(console, "error").mockImplementation(() => {});
    await AgentStore.actions.refresh();

    expect(AgentStore.state.items.map((agent) => agent.id)).toEqual(["api-builder"]);
  });
});
