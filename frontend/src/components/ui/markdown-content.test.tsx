import { describe, it, expect, vi } from "vitest";
import * as React from "react";
import { render } from "@testing-library/react";

// Pulled in through the inline chat tags; their generated avatar does not
// resolve under the test runner and is not what this is about.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarAgentFallback: () => null,
  AvatarFallback: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarImage: () => null,
}));

import { VirtualMarkdownRenderer } from "./markdown-content";

describe("markdown lists", () => {
  // Tailwind's preflight resets `list-style` on every list. Only the bullet
  // list got it back, so "1. … 2. …" rendered without numbers — and a reply
  // saying "I recommend option 1" pointed at nothing on screen.
  it("numbers an ordered list", () => {
    const { container } = render(<VirtualMarkdownRenderer content={"1. first\n2. second"} />);
    const list = container.querySelector("ol");
    expect(list).not.toBeNull();
    expect(list!.className).toMatch(/\blist-decimal\b/);
  });
});
