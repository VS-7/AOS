import { beforeEach, describe, expect, it, vi } from "vitest";

const wails = vi.hoisted(() => ({ openExternal: vi.fn(async () => {}), reloadHere: vi.fn() }));
vi.mock("@/lib/wails", () => wails);

import { ViewportStore } from "@/features/workspace/presentation/stores/viewport.store";
import { BrowserStore } from "@/features/workspace/presentation/stores/browser.store";
import { tabsGroup } from "./tabs.trigger";

type Stores = { viewport: typeof ViewportStore; browser: typeof BrowserStore };
type NavigateHandler = (args: { input?: unknown; stores: Stores }) => unknown;
const navigate = (url: string) =>
  (tabsGroup.build().triggers["tabs.navigate"].handler as unknown as NavigateHandler)({
    input: { url },
    stores: { viewport: ViewportStore, browser: BrowserStore },
  });
const activeTab = () =>
  ViewportStore.state.tabs.items.find((tab) => tab.id === ViewportStore.state.tabs.current);

describe("tabs.navigate", () => {
  beforeEach(() => {
    wails.openExternal.mockClear();
    ViewportStore.actions.createTab({ id: `browser-${Math.random()}` });
  });

  // A search was framed at duckduckgo.com, which refuses to be framed
  // (X-Frame-Options and frame-ancestors): every search was a blank tab under
  // "Open in your browser". Brave, Startpage and Mojeek refuse the same way.
  it("sends a search to the person's browser instead of framing a page that refuses", () => {
    BrowserStore.actions.setAddressBarValue("hello world");
    navigate("hello world");

    expect(wails.openExternal).toHaveBeenCalledWith("https://duckduckgo.com/?q=hello%20world");
    expect(activeTab()?.url).toBeUndefined();
    expect(BrowserStore.state.ui.addressBarValue).toBe("");
  });

  it("still opens an address in the tab", () => {
    navigate("example.com");

    expect(activeTab()?.url).toBe("https://example.com/");
    expect(wails.openExternal).not.toHaveBeenCalled();
  });
});
