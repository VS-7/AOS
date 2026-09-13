import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { DateTimeInput } from "./date-time-input";

afterEach(cleanup);

beforeAll(() => {
  // Radix's popper measures what it positions; jsdom has no layout.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
});

function open() {
  fireEvent.click(screen.getByRole("button", { name: /pick a date/i }));
}

// The calendar rendered without mode="single", and react-day-picker only
// makes days interactive when a mode or onDayClick is given: the days were
// plain text, clicking one did nothing, and a goal deadline could never be
// set or removed from the interface.
describe("DateTimeInput", () => {
  it("sets the day that was clicked", () => {
    const onValueChange = vi.fn();
    render(<DateTimeInput value={undefined} onValueChange={onValueChange} />);
    open();
    const day = screen.getAllByRole("button").find((button) => button.closest("td[data-day]"));
    expect(day, "a clickable day").toBeTruthy();
    fireEvent.click(day!);
    expect(onValueChange).toHaveBeenCalledTimes(1);
    expect(onValueChange.mock.calls[0][0]).toMatch(/^\d{4}-\d{2}-\d{2}T00:00:00\.000Z$/);
  });

  it("offers a way to remove a date that is set", () => {
    const onValueChange = vi.fn();
    render(<DateTimeInput value="2026-09-20T00:00:00.000Z" onValueChange={onValueChange} clearLabel="Remove deadline" />);
    fireEvent.click(screen.getByRole("button", { name: /2026/ }));
    fireEvent.click(screen.getByRole("button", { name: "Remove deadline" }));
    expect(onValueChange).toHaveBeenCalledWith(undefined);
  });
});
