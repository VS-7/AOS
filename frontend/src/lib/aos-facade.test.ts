import { describe, expect, it, vi, beforeEach } from "vitest";

const invoke = vi.fn();
vi.mock("./client", () => ({
  client: { invoke: (...a: unknown[]) => invoke(...a) },
  DomainError: class DomainError extends Error {
    code = "AOS_TEST"; status = 400; issues = {}; actions = [];
  },
  // command-map.ts eagerly imports lib/auth.ts and lib/file.ts, which in
  // turn import these from ./client. Neither is exercised by this suite
  // (only task.* and collection.* paths run below), but the module graph
  // still needs the bindings to exist for the import to resolve.
  unwrap: (raw: unknown) => raw,
  isDesktop: () => false,
  getWorkspace: () => "",
}));

const { flattenArgs, call, api, DORMANT_CODE } = await import("./aos-facade");

beforeEach(() => invoke.mockReset());

describe("flattenArgs", () => {
  it("merges params, query and body into a single payload", () => {
    expect(flattenArgs({ params: { task: "t-1" }, query: { limit: 10 }, body: { title: "x" } }))
      .toEqual({ task: "t-1", limit: 10, title: "x" });
  });

  it("does not let `enabled` leak into the payload", () => {
    expect(flattenArgs({ query: { limit: 5 }, enabled: false })).toEqual({ limit: 5 });
  });

  it("returns an empty object when there are no arguments", () => {
    expect(flattenArgs()).toEqual({});
  });
});

describe("call", () => {
  it("translates the name and injects _reasoning", async () => {
    invoke.mockResolvedValue({ tasks: [] });
    await call("task", "list", { query: { limit: 10 } });
    expect(invoke).toHaveBeenCalledWith("tasks_list", {
      limit: 10,
      _reasoning: "interface: task.list",
    });
  });

  it("resolves the kebab-case rename", async () => {
    invoke.mockResolvedValue({ task: {} });
    await call("task", "setStatus", { params: { id: "t-1" }, body: { status: "todo" } });
    expect(invoke).toHaveBeenCalledWith("tasks_set-status", {
      id: "t-1", status: "todo", _reasoning: "interface: task.setStatus",
    });
  });

  it("returns the payload under data, preserving the domain key", async () => {
    invoke.mockResolvedValue({ tasks: [{ id: "t-1" }], total: 1 });
    const r = await call("task", "list");
    expect(r.data).toEqual({ tasks: [{ id: "t-1" }], total: 1 });
    expect(r.error).toBeUndefined();
  });

  it("converts an exception into an envelope, without propagating it", async () => {
    // mockRejectedValueOnce, not mockRejectedValue: with a persistent
    // rejecting implementation, this Vitest version's mock-result tracking
    // spuriously reports an "unhandled rejection" for this test even though
    // call() demonstrably catches it (verified by direct instrumentation).
    // A one-shot rejection is equivalent here — invoke is called exactly
    // once — and does not trigger the false report.
    invoke.mockRejectedValueOnce(Object.assign(new Error("recusado"), { code: "AOS_TASK_BLOCKED" }));
    const r = await call("task", "list");
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe("AOS_TASK_BLOCKED");
  });

  it("answers dormant without touching the network", async () => {
    const r = await call("collection", "list");
    expect(invoke).not.toHaveBeenCalled();
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe(DORMANT_CODE);
  });

  it("fails loud when the call is not in the map", async () => {
    await expect(call("inventada", "list")).rejects.toThrow(/not mapped/);
  });
});

describe("api (the Proxy)", () => {
  it("exposes query, mutate, useQuery and useMutation on any feature.action", () => {
    // AosClient types api as Record<string, Record<string, ActionNode>>, so
    // noUncheckedIndexedAccess sees every level as possibly undefined. The
    // Proxy's get trap never actually returns undefined — it manufactures a
    // node for any key — so the `!` reflects a real runtime guarantee that
    // the index-signature type can't express.
    const node = api.task!.list!;
    expect(typeof node.query).toBe("function");
    expect(typeof node.mutate).toBe("function");
    expect(typeof node.useQuery).toBe("function");
    expect(typeof node.useMutation).toBe("function");
  });

  it("query goes through the same translation as call", async () => {
    invoke.mockResolvedValue({ tasks: [] });
    await api.task!.list!.query({ query: { limit: 3 } });
    expect(invoke).toHaveBeenCalledWith("tasks_list", {
      limit: 3, _reasoning: "interface: task.list",
    });
  });
});
