import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const listeners = vi.hoisted(() => new Map<string, (event: { data?: unknown }) => void>());
const restartDaemon = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());

vi.mock("@wailsio/runtime", () => ({
  Events: {
    On: (name: string, handler: (event: { data?: unknown }) => void) => {
      listeners.set(name, handler);
      return () => listeners.delete(name);
    },
  },
}));
vi.mock("@/lib/wails", () => ({ isDesktopWindow: true }));
vi.mock("@/lib/client", () => ({ DAEMON_EVENT: "aos:daemon", system: { restartDaemon } }));
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }));

import { VersionSkewBanner, skewOf } from "./version-skew-banner";

function daemonSays(data: unknown) {
  act(() => listeners.get("aos:daemon")?.({ data }));
}

beforeEach(() => {
  cleanup();
  listeners.clear();
  restartDaemon.mockReset();
  toastError.mockReset();
});

describe("skewOf", () => {
  it("reads the skew the window relays, in either shape Wails delivers it", () => {
    const skew = { window: "v0.15.0", daemon: "v0.15.2", windowOlder: true, compatible: true };
    expect(skewOf({ healthy: true, skew })).toEqual(skew);
    expect(skewOf([{ healthy: true, skew }])).toEqual(skew);
  });

  it("has none while the daemon matches, is away, or says nothing it can read", () => {
    expect(skewOf({ healthy: true })).toBeNull();
    expect(skewOf({ healthy: false, skew: { window: "a", daemon: "b" } })).toBeNull();
    expect(skewOf({ healthy: true, skew: { window: 1 } })).toBeNull();
    expect(skewOf(undefined)).toBeNull();
  });
});

// The defect: `aosd update apply` from a terminal replaced the daemon and the
// window's binary on disk, and the window that was open went on running the
// previous release against the new daemon with nothing saying so.
describe("VersionSkewBanner", () => {
  it("says nothing while the window and the daemon are one release", () => {
    render(<VersionSkewBanner />);
    daemonSays({ healthy: true });
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("tells the person to reopen a window the daemon was updated under", () => {
    render(<VersionSkewBanner />);
    daemonSays({ healthy: true, skew: { window: "v0.15.0", daemon: "v0.15.2", windowOlder: true, compatible: true } });

    const banner = screen.getByRole("status");
    expect(banner.textContent).toContain("v0.15.2");
    expect(banner.textContent).toMatch(/Quit AOS and open it again/);
    // Restarting the daemon would change nothing: the window is the old one.
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("restarts a daemon older than the window from the banner", async () => {
    restartDaemon.mockResolvedValue(undefined);
    render(<VersionSkewBanner />);
    daemonSays({ healthy: true, skew: { window: "v0.15.2", daemon: "v0.15.0", windowOlder: false, compatible: true } });

    expect(screen.getByRole("status").textContent).toMatch(/older than this window/);
    fireEvent.click(screen.getByRole("button", { name: /Restart daemon/ }));
    await waitFor(() => expect(restartDaemon).toHaveBeenCalledTimes(1));
  });

  it("says when the restart fails", async () => {
    restartDaemon.mockRejectedValue(new Error("the daemon could not be stopped"));
    render(<VersionSkewBanner />);
    daemonSays({ healthy: true, skew: { window: "v0.16.0", daemon: "v0.15.0", windowOlder: false, compatible: false } });

    expect(screen.getByRole("status").textContent).toMatch(/cannot work together/);
    fireEvent.click(screen.getByRole("button", { name: /Restart daemon/ }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("the daemon could not be stopped"));
  });

  it("gets out of the way when the daemon stops answering or comes back level", () => {
    render(<VersionSkewBanner />);
    daemonSays({ healthy: true, skew: { window: "v0.15.0", daemon: "v0.15.2", windowOlder: true, compatible: true } });
    expect(screen.getByRole("status")).toBeTruthy();

    daemonSays({ healthy: false });
    expect(screen.queryByRole("status")).toBeNull();

    daemonSays({ healthy: true, skew: { window: "v0.15.0", daemon: "v0.15.2", windowOlder: true, compatible: true } });
    daemonSays({ healthy: true });
    expect(screen.queryByRole("status")).toBeNull();
  });
});
