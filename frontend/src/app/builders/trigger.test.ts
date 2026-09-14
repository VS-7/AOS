import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, renderHook } from "@testing-library/react";

import { AosTrigger, AosTriggerGroup, keybindMatches, useGlobalKeybindings } from "./trigger";

function press(key: string, mods: { meta?: boolean; shift?: boolean; alt?: boolean; ctrl?: boolean } = {}) {
  const event = new KeyboardEvent("keydown", {
    key,
    metaKey: !!mods.meta,
    shiftKey: !!mods.shift,
    altKey: !!mods.alt,
    ctrlKey: !!mods.ctrl,
    cancelable: true,
  });
  window.dispatchEvent(event);
  return event;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("the palette's list", () => {
  // One group's loader read a store name that was never registered, threw,
  // and the rejection escaped list(): the palette said "No results found."
  // for every command in the application, on every keystroke.
  it("keeps every other group when one loader throws", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const built = AosTrigger.create()
      .addGroup(
        AosTriggerGroup.create("Broken").withLoader(() => {
          throw new TypeError("Cannot read properties of undefined (reading 'state')");
        }),
      )
      .addGroup(
        AosTriggerGroup.create("Goals").addTrigger({ id: "goals.new", label: "Create New Goal", handler: () => {} }),
      )
      .build();
    const api = built.initialize({ stores: {} } as never);

    const listed = await api.list({ query: "" });

    expect(listed.map((trigger) => trigger.id)).toEqual(["goals.new"]);
    expect(console.error).toHaveBeenCalled();
  });
});

describe("a keybind", () => {
  it("matches only the modifiers it names", () => {
    const event = (key: string, mods: KeyboardEventInit) => new KeyboardEvent("keydown", { key, ...mods });
    expect(keybindMatches("mod+n", event("n", { metaKey: true }))).toBe(true);
    expect(keybindMatches("mod+n", event("n", { ctrlKey: true }))).toBe(true);
    // ⌘⇧N is New Chat; it must not also open the New Task dialog.
    expect(keybindMatches("mod+n", event("N", { metaKey: true, shiftKey: true }))).toBe(false);
    expect(keybindMatches("mod+shift+n", event("N", { metaKey: true, shiftKey: true }))).toBe(true);
    expect(keybindMatches("mod+shift+n", event("n", { metaKey: true }))).toBe(false);
    expect(keybindMatches("mod+n", event("n", {}))).toBe(false);
    expect(keybindMatches("mod+left", event("ArrowLeft", { metaKey: true }))).toBe(true);
  });
});

describe("the application's keybindings", () => {
  function registry(handler: () => void) {
    return AosTrigger.create()
      .addGroup(
        AosTriggerGroup.create("Goals")
          .addTrigger({ id: "goals.new", label: "Create New Goal", keybind: "mod+shift+g", handler })
          .addTrigger({ id: "hidden.one", label: "Hidden", keybind: "mod+shift+i", hidden: true, handler })
          .addTrigger({ id: "owned.elsewhere", label: "Sidebar", keybind: "mod+b", globalKeybind: false, handler }),
      )
      .build();
  }

  // The palette listed ⌘⇧G, ⌘⇧P, ⌘N… and pressing them did nothing: a
  // keybind was only listened for while some mounted component happened to
  // call use() for that trigger, and nothing did for most of them.
  it("answers a listed shortcut no component has taken", () => {
    const built = registry(() => {});
    const run = vi.fn();
    renderHook(() => useGlobalKeybindings(built, run));

    const event = press("G", { meta: true, shift: true });

    expect(run).toHaveBeenCalledWith("goals.new");
    expect(event.defaultPrevented).toBe(true);
  });

  it("leaves hidden triggers and keybinds another component owns alone", () => {
    const built = registry(() => {});
    const run = vi.fn();
    renderHook(() => useGlobalKeybindings(built, run));

    press("I", { meta: true, shift: true });
    press("b", { meta: true });

    expect(run).not.toHaveBeenCalled();
  });

  // A component that calls use() decides for itself — when it is enabled,
  // and what the key does — so the global binding must not fire as well:
  // a toggle pressed once would toggle twice.
  it("defers to a mounted component that took the trigger", () => {
    const built = registry(() => {});
    const api = built.initialize({ stores: {} } as never);
    const run = vi.fn();
    const onPressKey = vi.fn();
    renderHook(() => useGlobalKeybindings(built, run));
    const claim = renderHook(() => api.use({ trigger: "goals.new", onPressKey }));

    press("G", { meta: true, shift: true });
    expect(onPressKey).toHaveBeenCalledTimes(1);
    expect(run).not.toHaveBeenCalled();

    claim.unmount();
    press("G", { meta: true, shift: true });
    expect(run).toHaveBeenCalledWith("goals.new");
  });
});
