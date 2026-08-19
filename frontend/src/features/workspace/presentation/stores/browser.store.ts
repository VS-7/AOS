import { AosStore } from "@/app/builders/store";

export const BROWSER_HOME_URL = "https://duckduckgo.com/";
const BROWSER_SEARCH_URL = "https://duckduckgo.com/?q=";

export interface BrowserNavigateResult {
  tabId: string;
  url: string;
}

export function normalizeBrowserUrl(value: string) {
  const input = value.trim();

  if (!input) {
    return BROWSER_HOME_URL;
  }

  try {
    return new URL(input).toString();
  } catch {
    if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(input)) {
      return input;
    }

    if (!input.includes(" ") && input.includes(".")) {
      try {
        return new URL(`https://${input}`).toString();
      } catch {
        return `${BROWSER_SEARCH_URL}${encodeURIComponent(input)}`;
      }
    }

    return `${BROWSER_SEARCH_URL}${encodeURIComponent(input)}`;
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
