import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { COMMAND_MAP } from "./command-map";
import * as authApi from "./auth";
import * as reactQuery from "@tanstack/react-query";

const invoke = vi.fn();
// Real react-query's useQuery needs a QueryClientProvider to run at all, and
// what's under test here is the config it's handed — queryFn's dormant/
// error/undefined-data handling — not react-query's own scheduling. Spread
// the real module (useMutation stays real, unused by these tests) and
// replace only useQuery with a spy that hands back its config untouched, so
// tests can call `config.queryFn()` directly.
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQuery: vi.fn((config: unknown) => config),
}));
// Spread the real module and override only `client`. command-map.ts eagerly
// imports lib/auth.ts and lib/file.ts, which pull in DomainError, unwrap,
// isDesktop and getWorkspace from ./client — neither module is exercised
// directly by this suite, but the import graph still needs those bindings
// to resolve. Re-stubbing each one by hand (as an earlier version of this
// file did) is both wrong (a hand-rolled `unwrap` that just returns its
// input isn't the real unwrap/DomainError contract, so a test that actually
// exercised those call sites could pass for the wrong reason) and brittle
// (any new export auth.ts or file.ts starts using from ./client breaks this
// suite with a cryptic "No 'X' export is defined on the './client' mock").
// Spreading the real module sidesteps both problems.
vi.mock("./client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./client")>()),
  client: { invoke: (...a: unknown[]) => invoke(...a) },
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

describe("call (HttpHandler branch)", () => {
  // auth.*, session.*, password.* and file.* — 13 entries — route through a
  // plain function instead of client.invoke (see command-map.ts's own
  // HttpHandler doc). Nothing exercised that branch before this suite: a
  // rename of a CallOpts key on either side of the auth.login entry, or a
  // regression that started leaking _reasoning into it, would have broken
  // silently. These tests close that gap.

  const ECHO_PATH = "test.echo";

  afterEach(() => {
    delete COMMAND_MAP[ECHO_PATH];
  });

  it("invokes the handler with exactly the flattened payload — no _reasoning added", async () => {
    // A synthetic entry, not a real domain one: this isolates call()'s own
    // dispatch contract (what object the handler receives) from any single
    // domain's argument-extraction logic, which is covered separately below.
    let received: Record<string, unknown> | undefined;
    COMMAND_MAP[ECHO_PATH] = async (p) => {
      received = p;
      return { ok: true };
    };

    const r = await call("test", "echo", { params: { id: "t-1" }, query: { limit: 5 } });

    expect(received).toEqual({ id: "t-1", limit: 5 });
    expect(received).not.toHaveProperty("_reasoning");
    expect(invoke).not.toHaveBeenCalled();
    expect(r.data).toEqual({ ok: true });
  });

  it("converts a handler rejection into an error envelope instead of throwing", async () => {
    COMMAND_MAP[ECHO_PATH] = async () => {
      throw Object.assign(new Error("nope"), { code: "AOS_ECHO_FAILED" });
    };

    const r = await call("test", "echo");

    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe("AOS_ECHO_FAILED");
  });

  it("resolves auth.login's identifier/password from the flattened payload — the real entry, not a stand-in", async () => {
    const login = vi.spyOn(authApi, "login").mockResolvedValueOnce({
      user: { id: "u-1", name: "Ada", username: "ada", email: "ada@example.com", role: "member" },
      expiresAt: "2030-01-01T00:00:00Z",
    });

    const r = await call("auth", "login", { params: { identifier: "ada@example.com", password: "secret" } });

    expect(login).toHaveBeenCalledWith("ada@example.com", "secret");
    expect(invoke).not.toHaveBeenCalled();
    expect(r.error).toBeUndefined();
    login.mockRestore();
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

describe("useQuery's queryFn", () => {
  // @tanstack/query-core throws "... data is undefined" if queryFn ever
  // resolves to undefined — so a dormant call, and a live call that
  // legitimately answers with no body, both have to land on `null` instead.
  // Without this, api.instruction.list.useQuery() (a dormant domain) would
  // put the query in *error* state and DORMANT_CODE would never reach the
  // UI at all — the exact panel it exists to trigger becomes unreachable.

  function lastQueryConfig(): { queryFn: () => Promise<unknown> } {
    const calls = vi.mocked(reactQuery.useQuery).mock.calls;
    const last = calls.at(-1);
    if (!last) throw new Error("useQuery was not called");
    // The real UseQueryOptions#queryFn takes a QueryFunctionContext this
    // suite never needs — call() ignores it — so this narrows to the shape
    // actually used here, same escape hatch as the CommandKey cast in
    // aos-facade.ts: through `unknown` first, since the two signatures
    // don't otherwise overlap enough for TS to allow it directly.
    return last[0] as unknown as { queryFn: () => Promise<unknown> };
  }

  it("resolves a dormant call to null data, not undefined", async () => {
    api.collection!.list!.useQuery();
    const { queryFn } = lastQueryConfig();
    await expect(queryFn()).resolves.toBeNull();
    expect(invoke).not.toHaveBeenCalled();
  });

  it("resolves a successful call with no body to null, not undefined", async () => {
    api.task!.list!.useQuery();
    const { queryFn } = lastQueryConfig();
    invoke.mockResolvedValueOnce(undefined);
    await expect(queryFn()).resolves.toBeNull();
  });

  it("passes real data through untouched", async () => {
    api.task!.list!.useQuery();
    const { queryFn } = lastQueryConfig();
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    await expect(queryFn()).resolves.toEqual({ tasks: [{ id: "t-1" }] });
  });

  it("throws a real Error carrying `code`, for a non-dormant failure", async () => {
    api.task!.list!.useQuery();
    const { queryFn } = lastQueryConfig();
    invoke.mockRejectedValueOnce(Object.assign(new Error("nope"), { code: "AOS_TASK_BLOCKED" }));
    await expect(queryFn()).rejects.toBeInstanceOf(Error);
  });

  it("keeps the error code reachable on the thrown Error", async () => {
    api.task!.list!.useQuery();
    const { queryFn } = lastQueryConfig();
    invoke.mockRejectedValueOnce(Object.assign(new Error("nope"), { code: "AOS_TASK_BLOCKED" }));
    await expect(queryFn()).rejects.toMatchObject({ code: "AOS_TASK_BLOCKED" });
  });
});
