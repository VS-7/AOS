import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import * as React from "react";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";

vi.mock("frimousse", () => {
  const Passthrough = ({ children }: { children?: React.ReactNode }) => <div>{typeof children === "function" ? null : children}</div>;
  return {
    EmojiPicker: {
      Root: ({ children, emojibaseUrl, locale }: { children?: React.ReactNode; emojibaseUrl?: string; locale?: string }) => (
        <div data-testid="picker" data-url={emojibaseUrl} data-locale={locale}>{children}</div>
      ),
      Search: () => <input />,
      Viewport: Passthrough,
      Loading: Passthrough,
      Empty: Passthrough,
      List: () => null,
    },
  };
});

import { ChatEmojiPicker, EMOJIBASE_URL } from "./emoji-picker";

const fetchMock = vi.fn();
beforeEach(() => {
  cleanup();
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("ChatEmojiPicker", () => {
  // The picker fetched emojibase-data@latest from a public CDN at runtime.
  // Offline, or with the CDN blocked, it said "Loading emojis..." forever.
  it("reads the emoji data the application ships, not a CDN", async () => {
    fetchMock.mockResolvedValue(new Response("{}", { status: 200 }));
    render(<ChatEmojiPicker onSelectEmoji={vi.fn()} />);

    const picker = await screen.findByTestId("picker");
    expect(picker.getAttribute("data-url")).toBe(EMOJIBASE_URL);
    expect(EMOJIBASE_URL.startsWith("/")).toBe(true);
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/^\/assets\/emojibase\/.+\/messages\.json$/);
  });

  it("says so, and offers a retry, when the data cannot be read", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    render(<ChatEmojiPicker onSelectEmoji={vi.fn()} />);

    await screen.findByText(/could not be loaded/i);
    expect(screen.queryByTestId("picker")).toBeNull();

    fetchMock.mockResolvedValueOnce(new Response("{}", { status: 200 }));
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    await waitFor(() => expect(screen.getByTestId("picker")).toBeTruthy());
  });
});
