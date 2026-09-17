import type React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";

vi.mock("@/app/aos", () => ({ aos: { client: { comment: {} } } }));
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm: vi.fn() }) }));
vi.mock("@/components/ui/markdown-content", () => ({
  MarkdownRenderer: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// The real avatar draws a hashvatar, whose ESM build does not load under the
// test runner; the item's text is what is asserted here.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarFallback: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  AvatarAgentFallback: () => <span />,
}));

import { CommentItem } from "./item.component";

// What comments_list answers: no attachments field at all.
const fromDaemon = {
  taskId: "0186fedb-0e6c-4856-bd81-ec695709f859",
  id: "0be9ad22-cf09-42d2-bc58-76ee304bef5a",
  author: "07da33a3-0314-4b68-92bf-3a8aa2b7647a",
  authorType: "user" as const,
  createdAt: "2026-08-30T13:50:00-03:00",
  updatedAt: "2026-08-30T13:50:00-03:00",
  content: "The worktree could not be created.",
};

const names: Record<string, string> = { "07da33a3-0314-4b68-92bf-3a8aa2b7647a": "Vitor" };
const props = {
  taskId: fromDaemon.taskId,
  onReply: vi.fn(),
  onChanged: vi.fn(),
  authorName: (author: string) => names[author] ?? author,
};

beforeEach(() => cleanup());

describe("CommentItem", () => {
  // Opening Comments on a task that had any took the whole task page to its
  // error boundary: "Cannot read properties of undefined (reading 'length')".
  it("renders a comment the daemon sent without attachments", () => {
    render(<CommentItem {...props} node={{ comment: fromDaemon, depth: 0, children: [] }} />);
    expect(screen.getByText("The worktree could not be created.")).toBeTruthy();
  });

  it("signs a person's comment with their name, not their id", () => {
    render(<CommentItem {...props} node={{ comment: fromDaemon, depth: 0, children: [] }} />);
    expect(screen.getByText("Vitor")).toBeTruthy();
    expect(screen.queryByText(fromDaemon.author)).toBeNull();
  });

  // The daemon lets an actor edit only what it wrote.
  it("offers Edit and Delete on the signed-in person's own comment only", () => {
    const { rerender } = render(
      <CommentItem {...props} selfId={fromDaemon.author} node={{ comment: fromDaemon, depth: 0, children: [] }} />,
    );
    expect(screen.getByRole("button", { name: /Edit/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Delete/ })).toBeTruthy();

    rerender(<CommentItem {...props} selfId="someone-else" node={{ comment: fromDaemon, depth: 0, children: [] }} />);
    expect(screen.queryByRole("button", { name: /Edit/ })).toBeNull();

    const agentComment = { ...fromDaemon, author: "api-builder", authorType: "agent" as const };
    rerender(<CommentItem {...props} selfId={fromDaemon.author} node={{ comment: agentComment, depth: 0, children: [] }} />);
    expect(screen.queryByRole("button", { name: /Delete/ })).toBeNull();
  });
});
