import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

const host = vi.hoisted(() => ({ platform: "darwin" }));
vi.mock("@/lib/wails", () => ({ platform: () => host.platform }));
vi.mock("@/hooks/use-mobile", () => ({ useIsMobile: () => false }));

import { SidebarProvider, useSidebar } from "./sidebar";

function State() {
  return <span data-testid="state">{useSidebar().state}</span>;
}

function press(key: string, mods: { meta?: boolean; ctrl?: boolean }) {
  const event = new KeyboardEvent("keydown", { key, metaKey: !!mods.meta, ctrlKey: !!mods.ctrl, cancelable: true, bubbles: true });
  act(() => {
    window.dispatchEvent(event);
  });
  return event;
}

afterEach(() => {
  cleanup();
  host.platform = "darwin";
});

describe("the sidebar's shortcut", () => {
  // ^B is "back one character" in every macOS text field; it collapsed the
  // sidebar instead, because Control counted as the command key.
  it("is ⌘B on macOS and leaves Control+B to the text field", () => {
    render(<SidebarProvider><State /></SidebarProvider>);

    const control = press("b", { ctrl: true });
    expect(control.defaultPrevented).toBe(false);
    expect(screen.getByTestId("state").textContent).toBe("expanded");

    press("b", { meta: true });
    expect(screen.getByTestId("state").textContent).toBe("collapsed");
  });

  it("is Control+B elsewhere", () => {
    host.platform = "linux";
    render(<SidebarProvider><State /></SidebarProvider>);

    press("b", { ctrl: true });
    expect(screen.getByTestId("state").textContent).toBe("collapsed");
  });
});
