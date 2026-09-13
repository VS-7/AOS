import { describe, expect, it, vi } from "vitest";

const bridge = vi.hoisted(() => ({ byName: vi.fn(async (..._args: unknown[]) => undefined) }));
vi.mock("@wailsio/runtime", () => ({
  Call: { ByName: bridge.byName },
  Browser: { OpenURL: vi.fn() },
  Clipboard: { SetText: vi.fn() },
  Dialogs: { Question: vi.fn() },
  System: { IsMac: () => false, IsWindows: () => false, IsLinux: () => false },
  Window: { Minimise: vi.fn(), ToggleMaximise: vi.fn(), Close: vi.fn(), IsMaximised: vi.fn() },
}));

/**
 * The theme store's modes are light, dark and system; SystemService.
 * SetAppearance (internal/transport/wailsvc/system.go) accepts light, dark and
 * auto. "system" was sent as it was, refused twice on every page load with
 * AOS_SYSTEM_UNKNOWN_APPEARANCE — and swallowed — so the native window never
 * followed the theme at all.
 */
describe("the appearance the native window is told", () => {
  it("speaks the bridge's vocabulary", async () => {
    const { nativeAppearance } = await import("./native");

    expect(nativeAppearance("light")).toBe("light");
    expect(nativeAppearance("dark")).toBe("dark");
    expect(nativeAppearance("system")).toBe("auto");
    expect(nativeAppearance("")).toBe("auto");
  });

  it("never sends the theme store's own word for it", async () => {
    window.history.replaceState({}, "", "/?daemon=http%3A%2F%2F127.0.0.1%3A7461&platform=darwin");
    vi.resetModules();
    const { installNativeBridge } = await import("./native");
    installNativeBridge();

    window.aos?.theme?.setAppearance?.({ mode: "system", windows: "blur" });
    await Promise.resolve();

    const call = bridge.byName.mock.calls.find((c) => String(c[0]).endsWith("SetAppearance"));
    expect(call?.slice(1)).toEqual(["auto", "blur"]);
    window.history.replaceState({}, "", "/");
  });
});
