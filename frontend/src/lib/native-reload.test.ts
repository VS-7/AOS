import { beforeEach, describe, expect, it, vi } from "vitest";

const runtime = vi.hoisted(() => ({
  handlers: new Map<string, () => void>(),
  order: [] as string[],
  emit: vi.fn(async (name: string) => {
    runtime.order.push(`emit ${name}`);
    return false;
  }),
}));
vi.mock("@wailsio/runtime", () => ({
  Call: { ByName: vi.fn(async () => undefined) },
  Browser: { OpenURL: vi.fn() },
  Events: {
    On: vi.fn((name: string, handler: () => void) => {
      runtime.handlers.set(name, handler);
      return () => {};
    }),
    Emit: runtime.emit,
  },
  Clipboard: { SetText: vi.fn() },
  Dialogs: { Question: vi.fn() },
  System: { IsMac: () => false, IsWindows: () => false, IsLinux: () => false },
  Window: { Minimise: vi.fn(), ToggleMaximise: vi.fn(), Close: vi.fn(), IsMaximised: vi.fn() },
}));
vi.mock("./wails", async (original) => ({
  ...(await original<typeof import("./wails")>()),
  reloadHere: vi.fn(() => runtime.order.push("reload")),
}));

/**
 * The native menu's View › Reload (Cmd+R) is sent to the page, which reloads
 * itself keeping the window's parameters. A bundle that failed before
 * installing its listener — a module error, a white window — left Cmd+R doing
 * nothing at all, and the application had to be restarted. The window now
 * reloads itself when the page does not say it heard; the page says so before
 * it reloads, while its bridge is still there to carry it.
 */
describe("the menu's Reload", () => {
  beforeEach(() => {
    // installNativeBridge installs once per window; each test is a fresh one.
    delete (window as { aos?: unknown }).aos;
    runtime.handlers.clear();
    runtime.order.length = 0;
  });

  it("is acknowledged to the window before the page reloads itself", async () => {
    window.history.replaceState({}, "", "/?daemon=http%3A%2F%2F127.0.0.1%3A7461&platform=darwin");
    vi.resetModules();
    const { installNativeBridge, RELOAD_EVENT, RELOAD_ACK_EVENT } = await import("./native");
    installNativeBridge();

    runtime.handlers.get(RELOAD_EVENT)?.();
    await vi.waitFor(() => expect(runtime.order).toContain("reload"));

    expect(RELOAD_ACK_EVENT).toBe("aos:reload-ack");
    expect(runtime.order).toEqual([`emit ${RELOAD_ACK_EVENT}`, "reload"]);
    window.history.replaceState({}, "", "/");
  });

  it("still reloads when the acknowledgement cannot be sent", async () => {
    runtime.emit.mockRejectedValueOnce(new Error("the bridge is gone"));
    window.history.replaceState({}, "", "/?daemon=http%3A%2F%2F127.0.0.1%3A7461&platform=darwin");
    vi.resetModules();
    const { installNativeBridge, RELOAD_EVENT } = await import("./native");
    installNativeBridge();

    runtime.handlers.get(RELOAD_EVENT)?.();
    await vi.waitFor(() => expect(runtime.order).toContain("reload"));
    window.history.replaceState({}, "", "/");
  });
});
