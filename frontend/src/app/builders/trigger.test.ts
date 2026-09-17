import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, renderHook } from "@testing-library/react";

const host = vi.hoisted(() => ({ platform: "darwin" }));
vi.mock("@/lib/wails", () => ({ platform: () => host.platform }));

import { AosTrigger, AosTriggerGroup, keybindMatches, useGlobalKeybindings } from "./trigger";

function press(
  key: string,
  mods: { meta?: boolean; shift?: boolean; alt?: boolean; ctrl?: boolean } = {},
  target: EventTarget = window,
) {
  const event = new KeyboardEvent("keydown", {
    key,
    metaKey: !!mods.meta,
    shiftKey: !!mods.shift,
    altKey: !!mods.alt,
    ctrlKey: !!mods.ctrl,
    cancelable: true,
    bubbles: true,
  });
  target.dispatchEvent(event);
  return event;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  host.platform = "darwin";
  document.body.innerHTML = "";
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
    // ⌘⇧N is New Chat; it must not also open the New Task dialog.
    expect(keybindMatches("mod+n", event("N", { metaKey: true, shiftKey: true }))).toBe(false);
    expect(keybindMatches("mod+shift+n", event("N", { metaKey: true, shiftKey: true }))).toBe(true);
    expect(keybindMatches("mod+shift+n", event("n", { metaKey: true }))).toBe(false);
    expect(keybindMatches("mod+n", event("n", {}))).toBe(false);
    expect(keybindMatches("mod+left", event("ArrowLeft", { metaKey: true }))).toBe(true);
  });

  // "mod" was ⌘ or Control on every platform. On macOS Control is text
  // editing — ^N moves down a line, ^K deletes to its end, ^B moves back — so
  // typing in a title field and pressing ^N opened Create Task.
  it("reads mod as the platform's command key: ⌘ on macOS, Control elsewhere", () => {
    const event = (key: string, mods: KeyboardEventInit) => new KeyboardEvent("keydown", { key, ...mods });
    expect(keybindMatches("mod+n", event("n", { ctrlKey: true }), true)).toBe(false);
    expect(keybindMatches("mod+n", event("n", { metaKey: true }), true)).toBe(true);
    expect(keybindMatches("mod+n", event("n", { ctrlKey: true }), false)).toBe(true);
    expect(keybindMatches("mod+n", event("n", { metaKey: true }), false)).toBe(false);
    expect(keybindMatches("ctrl+n", event("n", { ctrlKey: true }), true)).toBe(true);

    host.platform = "windows";
    expect(keybindMatches("mod+n", event("n", { ctrlKey: true }))).toBe(true);
    host.platform = "darwin";
    expect(keybindMatches("mod+n", event("n", { ctrlKey: true }))).toBe(false);
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

  it("leaves Control alone in a text field on macOS", () => {
    const built = registry(() => {});
    const run = vi.fn();
    renderHook(() => useGlobalKeybindings(built, run));
    const input = document.body.appendChild(document.createElement("input"));
    input.focus();

    const control = press("G", { ctrl: true, shift: true }, input);
    expect(run).not.toHaveBeenCalled();
    expect(control.defaultPrevented).toBe(false);

    press("G", { meta: true, shift: true }, input);
    expect(run).toHaveBeenCalledWith("goals.new");
  });

  // ⌘← and ⌘→ (Control on other systems) move the caret to the line's ends
  // or by word: in a field, that is what the person means, not Go Back.
  it("lets a text field keep the caret keys", () => {
    const back = vi.fn();
    const built = AosTrigger.create()
      .addGroup(AosTriggerGroup.create("Tabs").addTrigger({ id: "tabs.back", label: "Go Back", keybind: "mod+left", handler: back }))
      .build();
    const run = vi.fn();
    renderHook(() => useGlobalKeybindings(built, run));
    const editor = document.body.appendChild(document.createElement("div"));
    editor.setAttribute("contenteditable", "true");

    expect(press("ArrowLeft", { meta: true }, editor).defaultPrevented).toBe(false);
    expect(run).not.toHaveBeenCalled();

    press("ArrowLeft", { meta: true });
    expect(run).toHaveBeenCalledWith("tabs.back");
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
