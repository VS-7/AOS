"use client";

import * as React from "react";
import { aos } from "@/app/aos";

/**
 * Resolves the active visual theme mode from the ThemeStore.
 *
 * When the user preference is `system`, follows the OS color scheme.
 *
 * @returns The resolved theme mode (`light` or `dark`).
 */
export function useActiveThemeMode(): "light" | "dark" {
  const mode = aos.stores.theme.useState((s) => s.mode);

  return React.useMemo(() => {
    if (mode === "system") {
      return typeof window !== "undefined" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
    }

    return mode;
  }, [mode]);
}
