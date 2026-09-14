import { AosStore } from "@/app/builders/store";

// Where a search goes. It is not framed in the tab: DuckDuckGo refuses to be
// framed (X-Frame-Options and its own frame-ancestors), as Brave, Startpage
// and Mojeek do, so a search framed here was only ever a blank page under
// "Open in your browser". It opens in the person's browser instead.
const BROWSER_SEARCH_URL = "https://duckduckgo.com/?q=";

export interface BrowserNavigateResult {
  tabId: string;
  url: string;
}

/** What the address bar was given: a page to open in the tab, or a search. */
export type BrowserInput =
  | { kind: "address"; url: string }
  | { kind: "search"; url: string };

/**
 * Reads what was typed in the address bar.
 *
 * @param value - The address bar's text.
 * @returns The page to open, the search to run, or null for nothing typed.
 */
export function resolveBrowserInput(value: string): BrowserInput | null {
  const input = value.trim();

  if (!input) {
    return null;
  }

  const search: BrowserInput = {
    kind: "search",
    url: `${BROWSER_SEARCH_URL}${encodeURIComponent(input)}`,
  };

  try {
    return { kind: "address", url: new URL(input).toString() };
  } catch {
    if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(input)) {
      return { kind: "address", url: input };
    }

    if (!input.includes(" ") && input.includes(".")) {
      try {
        return { kind: "address", url: new URL(`https://${input}`).toString() };
      } catch {
        return search;
      }
    }

    return search;
  }
}

export const BrowserStore = AosStore.create("browser")
  .withState({
    ui: {
      addressBarValue: "",
      isAddressBarFocused: false,
    },
  })
  .addAction("setAddressBarValue", (ctx) => (value: string) =>
    ctx.state.set((state) => ({
      ui: {
        ...state.ui,
        addressBarValue: value,
      },
    }))
  )
  .addAction("setAddressBarFocused", (ctx) => (focused: boolean) =>
    ctx.state.set((state) => ({
      ui: {
        ...state.ui,
        isAddressBarFocused: focused,
      },
    }))
  )
  .addAction("focusAddressBar", (ctx) => () =>
    ctx.state.set((state) => ({
      ui: {
        ...state.ui,
        isAddressBarFocused: true,
      },
    }))
  )
  .withPersistence({ enabled: true, storage: "localstorage" })
  .withBroadcast({ enabled: true })
  .build();
