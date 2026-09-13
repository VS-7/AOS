import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { AgentModelSelect, type AgentModelSelectProvider } from "./agent-model-select";

/**
 * The picker's search box sits inside a Radix menu, and a menu treats every
 * printable key as typeahead: it moved focus to the first item whose label
 * started with the letter, so typing "anth" left "a" in the box. And the only
 * empty state it had said "No providers configured." — for a search miss, and
 * for a capability no connected provider serves, with a provider connected.
 */

beforeAll(() => {
  // Radix measures what it positions; jsdom has none of these.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never;
  Element.prototype.scrollIntoView ??= () => {};
  Element.prototype.hasPointerCapture ??= () => false;
});

afterEach(cleanup);

const providers: AgentModelSelectProvider[] = [
  {
    id: "anthropic",
    name: "Anthropic",
    configured: true,
    models: [
      { id: "claude-opus-5", name: "Claude Opus 5" },
      { id: "claude-haiku-4-5", name: "Claude Haiku 4.5" },
    ],
  },
  { id: "openai", name: "OpenAI", configured: true, models: [{ id: "gpt-5.1", name: "GPT-5.1" }] },
];

function open(props: Partial<React.ComponentProps<typeof AgentModelSelect>> = {}) {
  const onChange = vi.fn();
  render(
    <AgentModelSelect
      providers={providers}
      value={{ provider: "", model: "" }}
      onChange={onChange}
      showReasoning={false}
      defaultOpen
      {...props}
    />,
  );
  return { onChange, search: screen.getByPlaceholderText("Search models") as HTMLInputElement };
}

describe("AgentModelSelect", () => {
  it("keeps every typed key in the search box", () => {
    const { search } = open();
    search.focus();
    for (const key of "anth") {
      // A key the menu handles as typeahead never reaches the input.
      const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
      const stopped = vi.spyOn(event, "stopPropagation");
      search.dispatchEvent(event);
      expect(stopped, `"${key}" reached the menu's typeahead`).toHaveBeenCalled();
      fireEvent.change(search, { target: { value: search.value + key } });
    }
    expect(search.value).toBe("anth");
    expect(document.activeElement).toBe(search);
  });

  it("finds a model by its name or id, not only by its provider", () => {
    const { search, onChange } = open();
    fireEvent.change(search, { target: { value: "opus" } });

    const item = screen.getByRole("menuitem", { name: /Claude Opus 5/ });
    expect(screen.queryByRole("menuitem", { name: /GPT-5.1/ })).toBeNull();

    fireEvent.click(item);
    expect(onChange).toHaveBeenCalledWith({ provider: "anthropic", model: "claude-opus-5", reasoning: undefined });
  });

  it("says a search found nothing, not that nothing is configured", () => {
    const { search } = open();
    fireEvent.change(search, { target: { value: "zzz" } });

    expect(screen.getByText('No model matches "zzz".')).toBeTruthy();
    expect(screen.queryByText("No providers configured.")).toBeNull();
  });

  it("uses the caller's empty state when no provider can serve the slot", () => {
    open({ providers: [], emptyLabel: "No connected provider supports voice yet." });

    expect(screen.getByText("No connected provider supports voice yet.")).toBeTruthy();
  });

  it("offers to go back to the default when the caller allows it", () => {
    const onClear = vi.fn();
    open({ value: { provider: "openai", model: "gpt-5.1" }, onClear, clearLabel: "Use workspace default" });

    fireEvent.click(screen.getByRole("menuitem", { name: "Use workspace default" }));
    expect(onClear).toHaveBeenCalled();
  });
});
