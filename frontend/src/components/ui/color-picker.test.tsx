import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

import { ColorPickerPopover } from "./color-picker";

// jsdom has no layout, and the picker's sliders measure their track.
class NoLayout {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= NoLayout as unknown as typeof ResizeObserver;

beforeEach(() => {
  cleanup();
});

// Chooses a format the way a pointer does: the press lands first, then the
// click. The press is what the popover's outside-click check listens to.
function chooseFormat(label: string) {
  fireEvent.click(screen.getByRole("button", { expanded: false, name: /HEX/ }));
  const item = screen.getByRole("menuitemradio", { name: label });
  fireEvent.mouseDown(item);
  fireEvent.click(item);
}

describe("ColorPickerPopover", () => {
  // The workspace accent and the theme colours are stored as hex, and the
  // daemon refuses anything else. The format dropdown re-emitted the value as
  // rgb()/hsl()/oklch(), so switching it autosaved a colour the daemon
  // refused, and the window reported "the daemon is not answering".
  it("keeps a hex value hex when the channels are shown in another format", () => {
    const onValueChange = vi.fn();
    render(<ColorPickerPopover value="#5ed296" valueFormat="hex" onValueChange={onValueChange} />);

    fireEvent.click(screen.getByRole("button", { name: /5ED296/ }));
    chooseFormat("RGB");

    // The channels are now shown as RGB; the value did not change, so there
    // is nothing to save.
    expect(screen.getByRole("button", { name: /RGB/ })).toBeTruthy();
    expect(onValueChange).not.toHaveBeenCalled();

    // An edit made while RGB is shown is still reported as hex.
    fireEvent.keyDown(screen.getByLabelText("Red"), { key: "ArrowUp" });
    expect(onValueChange).toHaveBeenCalled();
    for (const [value] of onValueChange.mock.calls) {
      expect(value).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  // The menu is portalled outside the popover's panel, so pressing an item
  // counted as a click outside and closed the whole picker before the item
  // could be chosen: a pointer could never change the format.
  it("lets a pointer choose a format without closing the picker", () => {
    const onValueChange = vi.fn();
    render(<ColorPickerPopover value="#5ed296" onValueChange={onValueChange} />);

    fireEvent.click(screen.getByRole("button", { name: /5ED296/ }));
    chooseFormat("RGB");

    expect(onValueChange).toHaveBeenLastCalledWith("rgb(94, 210, 150)", expect.anything());
    expect(screen.getByRole("button", { name: /RGB/ })).toBeTruthy();
  });

  it("still closes when the press is outside both", () => {
    render(<ColorPickerPopover value="#5ed296" />);
    fireEvent.click(screen.getByRole("button", { name: /5ED296/ }));
    expect(screen.getByRole("button", { name: /HEX/ })).toBeTruthy();

    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("button", { name: /HEX/ })).toBeNull();
  });
});
