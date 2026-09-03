import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The Wails runtime, faked at the boundary this module actually uses it at.
 *
 * `System.IsMac` and friends are the synchronous platform reads; `Browser.
 * OpenURL` is the one call whose *absence* was the defect — every external
 * link in the desktop window went through `window.open`, which a WebView
 * answers with null and no window.
 */
const runtime = vi.hoisted(() => ({
  os: "" as string,
  openURL: vi.fn(async (_url: string) => {}),
  setText: vi.fn(async (_text: string) => {}),
  question: vi.fn(async (_options: unknown) => "Cancel"),
}));

vi.mock("@wailsio/runtime", () => ({
  Browser: { OpenURL: runtime.openURL },
  Clipboard: { SetText: runtime.setText },
  Dialogs: { Question: runtime.question },
  System: {
    IsMac: () => runtime.os === "darwin",
    IsWindows: () => runtime.os === "windows",
    IsLinux: () => runtime.os === "linux",
  },
  Window: {
    Minimise: vi.fn(),
    ToggleMaximise: vi.fn(),
    Close: vi.fn(),
    IsMaximised: vi.fn(async () => false),
  },
}));

/**
 * Loads `lib/wails` as if the page had just opened at `search`.
 *
 * The module reads the query string once at load, because the router rewrites
 * the URL on the first navigation — so each case has to be a fresh module.
 */
async function loadAt(search: string) {
  window.history.replaceState({}, "", `/${search}`);
  vi.resetModules();
  return import("./wails");
}

const DESKTOP = "?daemon=http%3A%2F%2F127.0.0.1%3A5326&platform=darwin";

beforeEach(() => {
  runtime.os = "";
  runtime.openURL.mockClear();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("knowing which window this is", () => {
  it("is the desktop only when the window said where the daemon is", async () => {
    expect((await loadAt(DESKTOP)).isDesktopWindow).toBe(true);
    expect((await loadAt("?welcome=true")).isDesktopWindow).toBe(false);
  });

  it("takes the platform from the URL before the Wails environment exists", async () => {
    // The host injects `window._wails.environment` from its
    // WebViewDidFinishNavigation hook, which is after this bundle has run and
    // already laid the window out. Until then `System.IsMac()` is false for
    // macOS and a browser tab alike.
    const wails = await loadAt(DESKTOP);
    expect(runtime.os).toBe("");
    expect(wails.platform()).toBe("darwin");
    expect(wails.isMac()).toBe(true);
  });

  it("prefers the Wails runtime once it has answered", async () => {
    const wails = await loadAt("?daemon=x&platform=linux");
    runtime.os = "windows";
    expect(wails.platform()).toBe("windows");
  });

  it("draws no window controls on macOS, and none at all in a browser", async () => {
    expect((await loadAt(DESKTOP)).needsWindowControls()).toBe(false);
    expect((await loadAt("?daemon=x&platform=windows")).needsWindowControls()).toBe(true);
    expect((await loadAt("?daemon=x&platform=linux")).needsWindowControls()).toBe(true);
    expect((await loadAt("")).needsWindowControls()).toBe(false);
  });
});

describe("keeping the window's identity across a hard navigation", () => {
  /**
   * The regression this exists for: switching workspace and finishing
   * onboarding both called `location.replace("/")`, which drops the query
   * string the desktop window was opened with. That string is the only thing
   * naming the daemon, so afterwards the window had no API origin, no event
   * channel and no `window.aos` — it re-entered itself as a browser tab and
   * stayed that way until the application was restarted.
   */
  it("carries the daemon address and platform onto the new URL", async () => {
    const wails = await loadAt(DESKTOP);
    const next = wails.desktopURL("/");

    expect(next).toContain("daemon=http%3A%2F%2F127.0.0.1%3A5326");
    expect(next).toContain("platform=darwin");
    expect(next.startsWith("/?")).toBe(true);
  });

  it("keeps a query string the caller asked for as well", async () => {
    const wails = await loadAt(DESKTOP);
    const next = wails.desktopURL("/?welcome=true");

    expect(next).toContain("welcome=true");
    expect(next).toContain("daemon=");
  });

  it("changes nothing in a browser, where a relative path is already right", async () => {
    const wails = await loadAt("?welcome=true");
    expect(wails.desktopURL("/")).toBe("/");
  });

  /**
   * The same defect, reached by the other door. `window.location.reload()`
   * re-runs the bundle at whatever the URL is *now*, and the router rewrote
   * it on the first navigation — so a plain reload dropped the daemon
   * address just as `replace("/")` did. Four places called it, including
   * the error screen's own Reload button: the surface meant for recovery was
   * the one that made the state unrecoverable.
   */
  it("reloading in place keeps the window's own parameters", async () => {
    const wails = await loadAt(DESKTOP);
    const real = window.location;
    const replace = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        // `desktopURL` resolves against href, so the stub carries one.
        href: "http://localhost/tasks",
        pathname: "/tasks",
        search: "",
        hash: "",
        replace,
      },
    });
    try {
      wails.reloadHere();
    } finally {
      // Restored, or every case after this one loads at a location that
      // cannot navigate.
      Object.defineProperty(window, "location", { configurable: true, value: real });
    }

    expect(replace).toHaveBeenCalledTimes(1);
    const next = String(replace.mock.calls[0][0]);
    expect(next.startsWith("/tasks?")).toBe(true);
    expect(next).toContain("daemon=http%3A%2F%2F127.0.0.1%3A5326");
    expect(next).toContain("platform=darwin");
  });
});

describe("opening a link outside the application", () => {
  it("hands the URL to the operating system in the desktop window", async () => {
    const wails = await loadAt(DESKTOP);
    await wails.openExternal("https://example.com/docs");

    expect(runtime.openURL).toHaveBeenCalledWith("https://example.com/docs");
  });

  it("uses a new tab in a browser, where that works", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const wails = await loadAt("");

    await wails.openExternal("https://example.com/docs");

    expect(runtime.openURL).not.toHaveBeenCalled();
    expect(open).toHaveBeenCalledWith(
      "https://example.com/docs",
      "_blank",
      "noopener,noreferrer",
    );
    open.mockRestore();
  });

  /**
   * There are around twenty `target="_blank"` anchors in this interface plus
   * every link inside a rendered markdown document — which is model output, so
   * the set is open-ended. All of them are dead in the desktop window: the
   * navigation needs the UI delegate's `createWebViewWithConfiguration`, which
   * Wails does not implement. One listener covers all of them.
   */
  it("intercepts an external anchor rather than letting the WebView drop it", async () => {
    const wails = await loadAt(DESKTOP);
    const remove = wails.installExternalLinkHandler();

    const anchor = document.createElement("a");
    anchor.href = "https://example.com/spec";
    anchor.target = "_blank";
    document.body.append(anchor);

    const event = new MouseEvent("click", { bubbles: true, cancelable: true });
    anchor.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(runtime.openURL).toHaveBeenCalledWith("https://example.com/spec");
    remove();
  });

  /**
   * The page's own origin is `wails://localhost` in a built application but
   * `http://localhost:9245` under `wails3 dev`. A rule of "does the href start
   * with http" is right in the first and catastrophic in the second: every
   * route in the interface would leave for the system browser. Same-origin is
   * the test, and this is the case that says so — jsdom serves this page over
   * http, exactly like the dev server does.
   */
  it("leaves a route inside the application to the router", async () => {
    const wails = await loadAt(DESKTOP);
    const remove = wails.installExternalLinkHandler();

    const anchor = document.createElement("a");
    anchor.href = "/tasks/42";
    document.body.append(anchor);

    expect(anchor.href.startsWith("http://")).toBe(true);

    const event = new MouseEvent("click", { bubbles: true, cancelable: true });
    anchor.dispatchEvent(event);

    // `defaultPrevented` is not the signal here: jsdom cancels a same-document
    // navigation it has not implemented, whoever asked for it. Whether the
    // operating system was handed the link is.
    expect(runtime.openURL).not.toHaveBeenCalled();
    remove();
  });

  it("defers to a component that is already handling the click itself", async () => {
    const wails = await loadAt(DESKTOP);
    const remove = wails.installExternalLinkHandler();

    const anchor = document.createElement("a");
    anchor.href = "https://example.com/handled";
    anchor.addEventListener("click", (e) => e.preventDefault());
    document.body.append(anchor);

    // The component's own listener is on the target, which runs after this
    // module's capture-phase one — so the guard that matters is the ordering
    // between two capture listeners. Pre-empt it the way a wrapper would.
    const event = new MouseEvent("click", { bubbles: true, cancelable: true });
    event.preventDefault();
    anchor.dispatchEvent(event);

    expect(runtime.openURL).not.toHaveBeenCalled();
    remove();
  });
});

describe("confirming something destructive", () => {
  /**
   * `window.confirm` returns false without drawing anything inside the desktop
   * window: WKWebView routes it to `runJavaScriptConfirmPanelWithMessage:`,
   * and Wails' UI delegate implements only the file-input panel. Four
   * destructive actions were guarded by one and were therefore unreachable.
   */
  it("asks through Wails rather than through window.confirm", async () => {
    const wails = await loadAt(DESKTOP);
    runtime.question.mockResolvedValueOnce("Delete");

    const accepted = await wails.confirmNatively({
      title: 'Delete "notes"?',
      message: "This cannot be undone.",
      confirmLabel: "Delete",
    });

    expect(accepted).toBe(true);
    expect(runtime.question).toHaveBeenCalledOnce();
  });

  it("treats a dialog that could not be shown as a refusal", async () => {
    const wails = await loadAt(DESKTOP);
    runtime.question.mockRejectedValueOnce(new Error("no window"));
    vi.spyOn(console, "error").mockImplementation(() => {});

    expect(await wails.confirmNatively({ title: "Delete?" })).toBe(false);
  });
});
