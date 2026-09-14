import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

const mutate = vi.hoisted(() => vi.fn());
const chats = vi.hoisted(() => ({ items: [] as unknown[] }));
vi.mock("@/app/aos", () => ({
  aos: {
    client: { chat: { create: { useMutation: () => ({ mutate, loading: false }) } } },
    stores: { chat: { useState: (select: (s: typeof chats) => unknown) => select(chats) } },
  },
}));
vi.mock("@/features/chat/presentation/helpers/open-chat-tab.helper", () => ({ openChatTab: vi.fn() }));
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { CreateChannelDialog } from "./create-channel-dialog";

/** Types one character at a time, the way a keyboard does: each keystroke lands on what the input shows now. */
function typeInto(input: HTMLInputElement, text: string) {
  for (const char of text) {
    fireEvent.change(input, { target: { value: input.value + char } });
  }
}

function openDialog() {
  render(<CreateChannelDialog />);
  fireEvent.click(screen.getByRole("button", { name: /create channel/i }));
  return screen.getByRole("textbox") as HTMLInputElement;
}

beforeEach(() => {
  cleanup();
  mutate.mockReset();
  chats.items = [];
});

describe("CreateChannelDialog", () => {
  // Slugifying every keystroke deleted the hyphen a space turns into before
  // the next letter arrived, so "time de produto" could only be typed as
  // "timedeproduto" — against the dialog's own "plano-orcamento" example.
  it("lets a multi-word name be typed and creates its slug", () => {
    const input = openDialog();
    typeInto(input, "time de produto");

    expect(input.value).toBe("time de produto");
    expect(screen.getByText(/#time-de-produto/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /^create$/i }));
    expect(mutate).toHaveBeenCalledWith({ body: { title: "time-de-produto" } });
  });

  it("keeps a typed hyphen", () => {
    const input = openDialog();
    typeInto(input, "time-de-produto");
    expect(screen.getByText(/#time-de-produto/)).toBeTruthy();
  });

  // Chats are keyed by id, so the daemon accepts a second channel with the
  // same name — and the sidebar then shows two identical rows nobody can
  // tell apart.
  it("refuses a name a channel already has", () => {
    chats.items = [{ id: "c-1", title: "time-de-produto", kind: "channel", participants: [] }];
    const input = openDialog();
    typeInto(input, "Time de Produto");

    expect(screen.getByText(/already a channel/i)).toBeTruthy();
    const create = screen.getByRole("button", { name: /^create$/i }) as HTMLButtonElement;
    expect(create.disabled).toBe(true);
  });
});
