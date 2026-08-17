import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";

/**
 * Minimal global client state: the resolved appearance and the visibility of
 * the two collapsible panels the ported layout shell (split-page-layout)
 * uses. Everything else server-shaped stays in TanStack Query, per
 * docs/06 - Frontend/React 19 e Bindings.md — this replaces the original's
 * Igniter.js store (`igniter.stores.theme`, `igniter.stores.viewport`), not a
 * new state library.
 */
interface AppState {
  appearance: "light" | "dark";
  sidebarVisible: boolean;
  detailsVisible: boolean;
  setSidebarVisible: (visible: boolean) => void;
  setDetailsVisible: (visible: boolean) => void;
}

const AppStateContext = createContext<AppState | null>(null);

function currentAppearance(): "light" | "dark" {
  return document.documentElement.classList.contains("light") ? "light" : "dark";
}

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [appearance, setAppearance] = useState<"light" | "dark">(currentAppearance);
  const [sidebarVisible, setSidebarVisible] = useState(true);
  const [detailsVisible, setDetailsVisible] = useState(true);

  useEffect(() => {
    // applyTheme (lib/theme.ts) sets the class directly on <html> rather than
    // through React, so components that need to react to it — a component the
    // theme picker itself does not render — observe the class instead of
    // threading the choice through props.
    const observer = new MutationObserver(() => setAppearance(currentAppearance()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return (
    <AppStateContext.Provider
      value={{ appearance, sidebarVisible, detailsVisible, setSidebarVisible, setDetailsVisible }}
    >
      {children}
    </AppStateContext.Provider>
  );
}

function useAppState(): AppState {
  const ctx = useContext(AppStateContext);
  if (!ctx) throw new Error("useAppState called outside <AppStateProvider>");
  return ctx;
}

/** The resolved appearance, reactive to the theme picker's choice. */
export function useAppearance(): "light" | "dark" {
  return useAppState().appearance;
}

/** Visibility of the split-page-layout's side panels. */
export function useLayout() {
  const { sidebarVisible, detailsVisible, setSidebarVisible, setDetailsVisible } = useAppState();
  return {
    sidebarVisible,
    detailsVisible,
    toggleSidebar: () => setSidebarVisible(!sidebarVisible),
    toggleDetails: () => setDetailsVisible(!detailsVisible),
    setSidebarVisible,
    setDetailsVisible,
  };
}
