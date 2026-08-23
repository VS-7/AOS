import * as React from "react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { COMMAND_MAP } from "./command-map";
import * as authApi from "./auth";

const invoke = vi.fn();

/**
 * Second pass, final review: `useQuery`'s "queryFn" suite used to fake
 * react-query's own `useQuery` with a spy that handed back its config
 * object untouched, so tests could call `config.queryFn()` directly without
 * a real render. That worked until `ActionNode.useQuery` (`aos-facade.ts`)
 * gained a real `React.useEffect` for the `onSuccess` shim — calling that
 * effect's owning function outside an actual render has no dispatcher to
 * call into (`Cannot read properties of null (reading 'useEffect')`), which
 * is exactly what broke (`git log -S "React.useEffect"` dates the effect to
 * `6ab2f34`, Task 9's bulk copy — the fake predates it and was never
 * updated once it landed). Rendering the real hook through a real
 * `QueryClientProvider` (`renderHook`, `@testing-library/react`) tests the
 * hook as what it actually is, including the effect the old fake couldn't
 * reach at all — the `onSuccess` shim had no coverage of its own before
 * this. `vite.config.ts`'s `test.environment` switched to `jsdom` for this.
 */
function withQueryClient() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

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

  it("M2: throws (as an envelope) instead of silently dropping data when two coerceIn fields collide on the same output key", async () => {
    // The real shape this can happen with: `workspace.update`'s `git` and
    // `worktrees` entries both return `{set: {...}}` — a naive
    // `Object.assign` merge would have `worktrees`'s result clobber
    // `git`'s. Today's three call sites each submit only one of
    // git/worktrees/tasks per call, so this hasn't fired live — this test
    // is what would catch a future call site combining them.
    const r = await call("workspace", "update", {
      params: { id: "w-1" },
      body: { git: { autoCommit: true }, worktrees: { enabled: true } },
    });
    expect(invoke).not.toHaveBeenCalled();
    expect(r.data).toBeUndefined();
    expect(r.error?.message).toMatch(/coerceIn collision/);
    expect(r.error?.message).toContain("git");
    expect(r.error?.message).toContain("worktrees");
  });

  it("config.update flattens a nested section into Go's dotted-path `set` — tunnel/index.tsx's real shape", async () => {
    // Before this coerceIn existed, `{tunnel: {...}}}` reached `config_update`
    // with no `set` key at all — `UpdateInput.Set` is `validate:"required"`,
    // so every tunnel settings save threw. This is the exact body `tunnel/
    // index.tsx`'s activation form submits.
    invoke.mockResolvedValue({ tunnel: { enabled: true } });
    await call("config", "update", { body: { tunnel: { enabled: true } } });
    expect(invoke).toHaveBeenCalledWith("config_update", {
      set: { "tunnel.enabled": true },
      _reasoning: "interface: config.update",
    });
  });

  it("config.update leaves agents.models as one whole-map leaf, not flattened into its slots", async () => {
    // `Agents.Models` is a `map[string]ModelRef` — a leaf per `patch.Apply`'s
    // own contract ("composite values are leaves: a caller replaces the
    // whole list"). `dotted` only flattens one level, so `agents.models`
    // itself stays a single key holding the whole map, the shape `agents/
    // index.tsx`'s `handleSlotChange` actually sends.
    invoke.mockResolvedValue({ agents: {} });
    await call("config", "update", {
      body: { agents: { models: { default: { provider: "openai", model: "gpt-5.1" } } } },
    });
    expect(invoke).toHaveBeenCalledWith("config_update", {
      set: { "agents.models": { default: { provider: "openai", model: "gpt-5.1" } } },
      _reasoning: "interface: config.update",
    });
  });

  it("answers dormant without touching the network", async () => {
    // The example moved, the behaviour did not: `collection` was the
    // stand-in dormant domain until task-10 lit it (along with `view`,
    // `toolset` and `skill`); `instruction` took over after that, until the
    // Phase 8 domain pass lit it too (along with `artifact`, `goal`,
    // `marketplace`, `project`, `template` and `tunnel`). `model` is still
    // whole-domain dormant — it is the same domain this file's `useQuery`
    // comment already names — so it inherits the job of proving the
    // short-circuit is real.
    const r = await call("model", "list");
    expect(invoke).not.toHaveBeenCalled();
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe(DORMANT_CODE);
  });

  it("answers a not-mapped call with an envelope, loudly logged, never a thrown exception", async () => {
    // B2 of the final review: this used to `throw` here, past the try/catch
    // — an unhandled rejection at any of the 117 call sites that read
    // envelopes with no try/catch of their own. `agent.getById` and
    // `memory.graph` both hid behind exactly this shape (a `.then().
    // finally()` chain with no `.catch()`) before they were mapped.
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const r = await call("inventada", "list");
    expect(r.data).toBeUndefined();
    expect(r.error?.code).toBe("AOS_CALL_NOT_MAPPED");
    expect(r.error?.message).toMatch(/not mapped/);
    expect(spy).toHaveBeenCalledWith(expect.stringMatching(/not mapped/));
    spy.mockRestore();
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
  //
  // These render the real hook (`renderHook`, `@testing-library/react`)
  // through a real `QueryClientProvider` — see this file's top-of-file
  // comment for why the earlier "fake useQuery, grab the config" approach
  // stopped working once the hook gained a real `React.useEffect`.

  it("resolves a dormant call to null data, not undefined", async () => {
    const { wrapper } = withQueryClient();
    // `model`, not `collection` or `instruction`: task-10 lit `collection`,
    // and the Phase 8 domain pass lit `instruction`, so neither exercises
    // the dormant branch this test exists for any more.
    const { result } = renderHook(() => api.model!.list!.useQuery(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeNull();
    expect(invoke).not.toHaveBeenCalled();
  });

  it("resolves a successful call with no body to null, not undefined", async () => {
    invoke.mockResolvedValueOnce(undefined);
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeNull();
  });

  it("passes real data through untouched", async () => {
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual({ tasks: [{ id: "t-1" }] });
  });

  it("throws a real Error carrying `code`, for a non-dormant failure", async () => {
    invoke.mockRejectedValueOnce(Object.assign(new Error("nope"), { code: "AOS_TASK_BLOCKED" }));
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(Error);
  });

  it("keeps the error code reachable on the thrown Error", async () => {
    invoke.mockRejectedValueOnce(Object.assign(new Error("nope"), { code: "AOS_TASK_BLOCKED" }));
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toMatchObject({ code: "AOS_TASK_BLOCKED" });
  });
});

describe("useQuery's onSuccess shim", () => {
  // No coverage existed for this before the final review's second pass —
  // it arrived in Task 9's bulk copy (`6ab2f34`) and was never looked at on
  // its own. See `aos-facade.ts`'s own comment on the shim for the "is this
  // a stale-closure bug" investigation this suite backs up: it isn't one,
  // but the structural-sharing gap below is real.

  it("fires once, with the resolved data, the first time the query succeeds", async () => {
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    const onSuccess = vi.fn();
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery({ onSuccess }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));

    expect(onSuccess).toHaveBeenCalledWith({ tasks: [{ id: "t-1" }] });
  });

  it("does not fire for a dormant resolution's null data on its own merits — but it does fire, since null is still a successful result", async () => {
    // Documents the actual, current behavior rather than assuming it:
    // dormant resolves `isSuccess: true` with `data: null` (see the queryFn
    // tests above), and this shim's only condition is `result.isSuccess` —
    // it does not special-case dormant. A dormant screen's `onSuccess`, if
    // it has one, does get called once with `null`.
    //
    // `model`, not `collection` or `instruction`: task-10 lit `collection`,
    // and the Phase 8 domain pass lit `instruction`. Either kept passing
    // after its own move — a live call with no `invoke` mock also resolves
    // to `null` — so it would silently stop being about dormancy at all.
    // The `invoke` assertion below is what makes the next such move fail
    // loudly instead of drifting.
    const onSuccess = vi.fn();
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.model!.list!.useQuery({ onSuccess }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));

    expect(invoke).not.toHaveBeenCalled();
    expect(onSuccess).toHaveBeenCalledWith(null);
  });

  it("fires again when a refetch resolves to genuinely different data", async () => {
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    const onSuccess = vi.fn();
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery({ onSuccess }), { wrapper });

    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));

    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }, { id: "t-2" }] });
    await result.current.refetch();

    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(2));
    expect(onSuccess).toHaveBeenLastCalledWith({ tasks: [{ id: "t-1" }, { id: "t-2" }] });
  });

  it("does NOT fire again when a refetch resolves to deep-equal data — react-query's structural sharing keeps the same reference, so the effect's deps don't change (the documented gap vs. real v4, which fires unconditionally per fetch)", async () => {
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    const onSuccess = vi.fn();
    const { wrapper } = withQueryClient();
    const { result } = renderHook(() => api.task!.list!.useQuery({ onSuccess }), { wrapper });

    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));

    // Deep-equal to the first response, but a fresh object — exactly what a
    // background refetch that found nothing new would resolve.
    invoke.mockResolvedValueOnce({ tasks: [{ id: "t-1" }] });
    await result.current.refetch();
    await waitFor(() => expect(result.current.isFetching).toBe(false));

    // Give any effect a chance to run before asserting it didn't.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(onSuccess).toHaveBeenCalledTimes(1);
  });
});
