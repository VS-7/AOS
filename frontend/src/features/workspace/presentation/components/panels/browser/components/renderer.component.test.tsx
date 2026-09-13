import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";

/**
 * An artifact opened in the desktop window is framed opaque, so its own
 * stylesheet, script and images are cross-origin loads — and the window
 * answers those only at the address it hands out for that artifact. Framed at
 * its plain /v/artifacts/ URL, it rendered as unstyled, inert HTML.
 */

const DESKTOP = "/?daemon=http%3A%2F%2F127.0.0.1%3A7461&platform=darwin";

async function renderAt(page: string, url: string) {
  window.history.replaceState({}, "", page);
  sessionStorage.clear();
  vi.resetModules();
  const { BrowserRenderer } = await import("./renderer.component");
  const onStateChange = vi.fn();
  const view = render(
    <BrowserRenderer tab={{ id: "t1", type: "browser", title: "Sales", url }} active onStateChange={onStateChange} />,
  );
  return { view, onStateChange };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.replaceState({}, "", "/");
});

describe("an artifact's frame", () => {
  it("is framed, opaque, at the address the window hands out", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ url: "/v/frame/g/sales/" }), { status: 200 })),
    );
    const { view } = await renderAt(DESKTOP, "/v/artifacts/sales/");

    await waitFor(() => expect(view.container.querySelector("iframe")).not.toBeNull());
    const frame = view.container.querySelector("iframe")!;
    expect(frame.getAttribute("src")).toBe("/v/frame/g/sales/");
    expect(frame.getAttribute("sandbox")).not.toContain("allow-same-origin");
  });

  it("says so, instead of framing the address that cannot load its files", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("no", { status: 404 })));
    vi.spyOn(console, "error").mockImplementation(() => {});
    const { view, onStateChange } = await renderAt(DESKTOP, "/v/artifacts/sales/");

    await waitFor(() =>
      expect(onStateChange).toHaveBeenCalledWith("t1", expect.objectContaining({ error: expect.any(String) })),
    );
    expect(view.container.querySelector("iframe")).toBeNull();
  });

  it("frames an artifact in a browser tab where it is", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { view } = await renderAt("/", "/v/artifacts/sales/");

    await waitFor(() => expect(view.container.querySelector("iframe")).not.toBeNull());
    expect(view.container.querySelector("iframe")!.getAttribute("src")).toBe("/v/artifacts/sales/");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
