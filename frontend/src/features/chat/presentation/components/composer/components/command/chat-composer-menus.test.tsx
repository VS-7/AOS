import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import * as React from "react";
import { render, screen, cleanup } from "@testing-library/react";
import { Popover, PopoverAnchor } from "@/components/ui/popover";

// The generated avatar's package does not resolve under the test runner, and
// who is drawn is not what these tests are about.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarAgentFallback: () => null,
  AvatarFallback: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarImage: () => null,
}));

import {
  CHAT_COMPOSER_SLASH_COMMANDS,
  ChatComposerSkillsCommandMenu,
} from "./chat-composer-skills-command-menu";
import { ChatComposerCommandMenu } from "./chat-composer-command-menu";

beforeAll(() => {
  // cmdk scrolls the active item into view; jsdom has no layout to do it with.
  Element.prototype.scrollIntoView = vi.fn();
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never;
});
beforeEach(() => cleanup());

function inPopover(node: React.ReactNode) {
  return render(
    <Popover open>
      <PopoverAnchor />
      {node}
    </Popover>,
  );
}

const commandRef = { current: null };
const file = { id: "f-1", kind: "file" as const, label: "notes.md", prompt: "", searchableValue: "notes.md" };
const luara = { kind: "agent" as const, key: "agent:luara", mentionId: "luara", label: "Luara" };

describe("the / menu", () => {
  // cmdk draws a group's heading even with nothing under it, so a workspace
  // with no skills showed a "Skills" heading over an empty space.
  it("has no Skills heading when there are no skills", () => {
    inPopover(
      <ChatComposerSkillsCommandMenu
        commandRef={commandRef}
        onCommandSelect={vi.fn()}
        onQueryChange={vi.fn()}
        onReferenceSelect={vi.fn()}
        query=""
        slashCommands={[CHAT_COMPOSER_SLASH_COMMANDS.STOP, CHAT_COMPOSER_SLASH_COMMANDS.CLEAR]}
        selectableSkills={[]}
        trigger={null}
      />,
    );
    expect(screen.getByText("Stop current execution")).toBeTruthy();
    expect(screen.queryByText("Skills")).toBeNull();
  });
});

describe("the + and @ menu", () => {
  const props = {
    commandMentionTargets: [luara],
    commandQuery: "",
    commandRef,
    isDirectMessage: false,
    onMentionSelect: vi.fn(),
    onQueryChange: vi.fn(),
    onReferenceSelect: vi.fn(),
    selectableFiles: [file],
  };

  // `@` is a mention. It opened the same menu as `+`, with an upload row and
  // the workspace's files ahead of the people it was meant to list.
  it("lists only who can be mentioned after an @", () => {
    inPopover(
      <ChatComposerCommandMenu {...props} mentionState={{ query: "", range: { start: 0, end: 1 } }} />,
    );
    expect(screen.getByText("Luara")).toBeTruthy();
    expect(screen.queryByText("notes.md")).toBeNull();
    expect(screen.queryByText(/upload/i)).toBeNull();
  });

  // Nothing attached reaches the agent (chats_send has no field for it), so
  // the menu does not offer an upload it would silently drop.
  it("offers no upload when attachments cannot be delivered", () => {
    inPopover(<ChatComposerCommandMenu {...props} mentionState={null} />);
    expect(screen.getByText("notes.md")).toBeTruthy();
    expect(screen.queryByText(/upload/i)).toBeNull();
  });
});
