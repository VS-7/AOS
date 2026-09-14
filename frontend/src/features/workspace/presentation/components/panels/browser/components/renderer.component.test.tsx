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

/**
 * A new tab used to open https://duckduckgo.com/, which refuses to be framed:
 * every new tab was the browser's broken-page icon with nothing said. And no
 * external site that refuses framing can be told apart from one that loaded —
 * a blocked frame still fires `load` — so the way out has to be always there.
 */
describe("a browser tab", () => {
  it("starts on a page of its own when it has no address yet", async () => {
    const { view } = await renderAt("/", "");

    expect(view.container.querySelector("iframe")).toBeNull();
    expect(view.getByText("Type an address or a search in the bar above.")).toBeTruthy();
  });

  it("offers the system browser for an external site", async () => {
    const { view } = await renderAt("/", "https://example.com/docs");
    // After renderAt, which resets the module graph: the same instance the
    // renderer imported.
    const wails = await import("@/lib/wails");
    const openExternal = vi.spyOn(wails, "openExternal").mockResolvedValue();

    await waitFor(() => expect(view.container.querySelector("iframe")).not.toBeNull());
    view.getByRole("button", { name: "Open in your browser" }).click();
    expect(openExternal).toHaveBeenCalledWith("https://example.com/docs");
  });

  it("does not offer it for the window's own content", async () => {
    vi.stubGlobal("fetch", vi.fn());
    const { view } = await renderAt("/", "/v/artifacts/sales/");

    await waitFor(() => expect(view.container.querySelector("iframe")).not.toBeNull());
    expect(view.queryByRole("button", { name: "Open in your browser" })).toBeNull();
  });
});

