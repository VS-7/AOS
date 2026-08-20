import { ViewportTabState } from "@/features/workspace/presentation/stores/viewport.store";

export function getTabLabel(tab: ViewportTabState) {
  if (tab.title && tab.title !== "New tab") return tab.title;

  try {
    if (tab.url) {
      const url = new URL(tab.url);
      return url.hostname.replace(/^www\./, "");
    }
    return "New tab";
  } catch {
    return "New tab";
  }
}