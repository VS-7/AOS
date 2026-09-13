import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, cleanup } from "@testing-library/react";
import * as React from "react";
import { DefaultTheme } from "@/features/theme/presentation/const/default-theme";

const setAppearance = vi.hoisted(() => vi.fn());

vi.mock("@/app/aos", () => ({
  aos: {
    stores: {
      theme: {
        useState: () => ({
          mode: "dark",
          radius: "lg",
          theme: { preset: "aos", settings: DefaultTheme.theme },
          fontSizes: { ui: 13, code: 12 },
        }),
      },
    },
  },
}));
vi.mock("@/lib/wails", () => ({ platform: () => "darwin" }));

import { ThemeProvider } from "./theme-provider";

beforeEach(() => {
  cleanup();
  setAppearance.mockClear();
  (window as unknown as { aos: unknown }).aos = { theme: { setAppearance } };
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  })) as unknown as typeof window.matchMedia;
});

// The provider now sits above the sign-in screens as well as inside the
// workspace layout — Login and Onboarding rendered outside it, on the dark
// defaults of tokens.css with an orange accent. Inside both, the inner one must
// not apply the theme a second time.
describe("ThemeProvider", () => {
  it("applies the theme once when nested", () => {
    const { container } = render(
      <ThemeProvider>
        <ThemeProvider>
          <span>app</span>
        </ThemeProvider>
      </ThemeProvider>,
    );
    expect(setAppearance).toHaveBeenCalledTimes(1);
    expect(container.querySelectorAll("style")).toHaveLength(1);
    expect(document.documentElement.style.getPropertyValue("--primary")).not.toBe("");
  });
});
