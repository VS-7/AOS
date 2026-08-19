"use client";

import * as React from "react";
import { aos } from "@/app/aos";
import type { FractalModelProvider } from "@/features/model/interfaces/model.interfaces";

/**
 * Resolves the provider logo URL for the active theme mode.
 *
 * The provider descriptor carries both `logo.light` and `logo.dark` URLs
 * (the backend already picked them). We still need to know which mode is
 * currently active in the UI (light / dark / system -> follows OS) to pick
 * the right one.
 */
export function useProviderLogo(provider: Pick<FractalModelProvider, "logo">) {
  const themeMode = aos.stores.theme.useState((s) => s.mode);

  return React.useMemo(() => {
    const isDark =
      themeMode === "dark" ||
      (themeMode === "system" &&
        typeof window !== "undefined" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);

    return isDark ? provider.logo.dark : provider.logo.light;
  }, [themeMode, provider.logo.dark, provider.logo.light]);
}
