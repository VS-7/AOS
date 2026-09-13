import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient } from "@tanstack/react-query";

/**
 * The desktop window's transports, driven through a fake bridge.
 *
 * Every one of these used to end the same way: whatever the bridge said, the
 * page tried again over plain HTTP from `wails://localhost`, with no
 * credential. A "no conversation with that id" became "this request carries
 * no valid credential" (and sent AuthGate round in a loop that unmounted the
 * whole application), or "the daemon is not answering" with the red banner —
 * the screen showed the retry's failure instead of the real one. One wrong
 * password was checked six times over five seconds for the same reason.
 *
 * The credential lives in the Go process. Inside the window the bridge's
 * answer is the answer.
 */

const bridge = vi.hoisted(() => ({ byName: vi.fn() }));
vi.mock("@wailsio/runtime", () => ({
  Call: { ByName: bridge.byName },
  Events: { On: () => () => {} },
  System: { IsMac: () => false, IsWindows: () => false, IsLinux: () => false },
}));

const DESKTOP = "/?daemon=http%3A%2F%2F127.0.0.1%3A7461&platform=darwin";

/** A Go error from a bound method, as `@wailsio/runtime` rejects with it. */
function runtimeError(code: string, message: string): Error {
  const err = new Error(`${code}: ${message}`);
  err.name = "RuntimeError";
  (err as Error & { cause: unknown }).cause = { code, causer: "test", message };
  return err;
}

async function loadAt(url: string) {
  window.history.replaceState({}, "", url);
  try {
    sessionStorage.clear();
  } catch {
    // Not every environment has one; the modules cope without it.
  }
  vi.resetModules();
  const client = await import("./client");
  const auth = await import("./auth");
  const file = await import("./file");
  const realtime = await import("./realtime");
  return { client, auth, file, realtime };
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  bridge.byName.mockReset();
  fetchMock = vi.fn(() => Promise.reject(new TypeError("Load failed")));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  window.history.replaceState({}, "", "/");
});

describe("a command inside the desktop window", () => {
  it("surfaces the daemon's refusal and never retries it over HTTP", async () => {
    const { client } = await loadAt(DESKTOP);
    bridge.byName.mockResolvedValue(
      JSON.stringify({ error: { code: "AOS_CHAT_NOT_FOUND", message: 'no conversation "x"' } }),
    );

    await expect(client.client.invoke("chats_get", { chat: "x" } as never)).rejects.toMatchObject({
      code: "AOS_CHAT_NOT_FOUND",
    });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(bridge.byName).toHaveBeenCalledTimes(1);
  });

  it("reads a Go error from the bridge as the domain error it carries", async () => {
    const { client } = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValue(runtimeError("AOS_DESKTOP_PATH_NOT_BRIDGED", "not bridged"));

    await expect(client.client.invoke("chats_get", {} as never)).rejects.toMatchObject({
      name: "DomainError",
      code: "AOS_DESKTOP_PATH_NOT_BRIDGED",
    });
    expect(bridge.byName).toHaveBeenCalledTimes(1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reports a bridge that never answered as unreachable, not as an HTTP failure", async () => {
    vi.useFakeTimers();
    const { client } = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValue(new TypeError("Load failed"));

    const outcome = client.client.invoke("chats_list", {} as never).catch((err: unknown) => err);
    await vi.advanceTimersByTimeAsync(10_000);

    expect(client.isDaemonUnreachable(await outcome)).toBe(true);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("the credential being refused", () => {
  // The daemon answers AOS_AUTH_UNAUTHENTICATED for an expired or revoked
  // bearer (httpapi's authenticate middleware passes auth.Service's error
  // through) and AOS_HTTP_UNAUTHENTICATED only when there is no bearer at
  // all. Recognising only the second left a revoked session on empty screens
  // with no way back to Login once the fallback stopped disguising it.
  it.each(["AOS_AUTH_UNAUTHENTICATED", "AOS_HTTP_UNAUTHENTICATED"])("announces %s", async (code) => {
    const { client } = await loadAt(DESKTOP);
    const heard = vi.fn();
    window.addEventListener(client.UNAUTHENTICATED_EVENT, heard);
    bridge.byName.mockResolvedValue(JSON.stringify({ error: { code, message: "no" } }));

    await expect(client.client.invoke("goals_list", {} as never)).rejects.toMatchObject({ code });

    expect(heard).toHaveBeenCalledTimes(1);
    window.removeEventListener(client.UNAUTHENTICATED_EVENT, heard);
  });

  it("reads the call to action and the issue under the names the daemon sends", async () => {
    const { client } = await loadAt("/");
    const error = new client.DomainError({
      code: "AOS_X",
      message: "m",
      issue: { path: "a" },
      cta: [{ label: "do this" }],
    } as never);

    expect(error.issues).toEqual({ path: "a" });
    expect(error.actions).toEqual([{ label: "do this" }]);
  });
});

describe("signing in inside the desktop window", () => {
  it("checks a wrong password once and shows the daemon's answer", async () => {
    const { auth } = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValue(
      runtimeError("AOS_AUTH_INVALID_CREDENTIALS", "those credentials do not match an account"),
    );

    await expect(auth.login("harness@local.test", "wrong")).rejects.toMatchObject({
      code: "AOS_AUTH_INVALID_CREDENTIALS",
    });
    expect(bridge.byName).toHaveBeenCalledTimes(1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // A bridge that answered "the daemon is down" is a warm bridge. Retrying
  // it here is what kept the window blank for five seconds before AuthGate
  // could say it was waiting — AuthGate has its own backoff for exactly this.
  it("hands a daemon that is not answering straight back to the caller", async () => {
    const { auth, client } = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValue(runtimeError("AOS_DAEMON_UNREACHABLE", "the daemon did not answer"));

    const outcome = await auth.status().catch((err: unknown) => err);

    expect(client.isDaemonUnreachable(outcome)).toBe(true);
    expect(bridge.byName).toHaveBeenCalledTimes(1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reaches the account roster through the bridge only", async () => {
    const { auth } = await loadAt(DESKTOP);
    bridge.byName.mockResolvedValue({ status: 200, body: JSON.stringify({ data: { users: [] } }) });

    await expect(auth.users()).resolves.toEqual({ users: [] });
    expect(bridge.byName.mock.calls[0]?.[0]).toMatch(/DomainService\.Fetch$/);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("the file surface", () => {
  it("does not fall back to a credential-less fetch inside the window", async () => {
    const { file } = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValue(runtimeError("AOS_DAEMON_UNREACHABLE", "the daemon did not answer"));

    const outcome = await file.tree("", true).catch((err: unknown) => err);

    expect(outcome).toMatchObject({ code: "AOS_DAEMON_UNREACHABLE" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("classifies a refused connection in a browser tab instead of throwing a bare TypeError", async () => {
    const { file, client } = await loadAt("/");

    const outcome = await file.tree("", true).catch((err: unknown) => err);

    expect(client.isDaemonUnreachable(outcome)).toBe(true);
    expect(bridge.byName).not.toHaveBeenCalled();
  });
});

describe("a browser tab", () => {
  it("never probes a bridge it does not have", async () => {
    const { auth } = await loadAt("/");
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ data: { onboarded: true, authenticated: true } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(auth.status()).resolves.toEqual({ onboarded: true, authenticated: true });
    expect(bridge.byName).not.toHaveBeenCalled();
  });
});

describe("the event channel in a browser tab", () => {
  // The page's own origin is the daemon there. Asking the bridge where the
  // daemon is cost a rejected POST /wails/runtime (a 404 from the daemon) on
  // every page load before the socket opened.
  it("opens against the page's own origin without asking a bridge", async () => {
    const { client, realtime } = await loadAt("/");
    const opened: string[] = [];
    vi.stubGlobal(
      "WebSocket",
      class {
        constructor(url: string) {
          opened.push(url);
        }
        close() {}
      },
    );
    await client.setWorkspace("vs");

    const { unmount } = renderHook(() => realtime.useRealtime(new QueryClient()));
    await vi.waitFor(() => expect(opened).toHaveLength(1));
    unmount();

    expect(opened[0]).toBe(`ws://${window.location.host}/ws?workspace=vs`);
    expect(bridge.byName).not.toHaveBeenCalled();
  });
});
