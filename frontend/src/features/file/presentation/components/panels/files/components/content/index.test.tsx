import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

/**
 * The file editor: what it seeds its buffer from, when it lets that buffer be
 * written back, and what it does when the workspace changes underneath it.
 */

let realtimeHandler: ((payload: unknown) => void) | undefined;
vi.mock("@/hooks/use-realtime", () => ({
  useRealtime: (event: string, callback: (payload: unknown) => void) => {
    if (event === "files:changed") realtimeHandler = callback;
  },
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn(), message: vi.fn() } }));

type ReadState = {
  data?: unknown;
  isError?: boolean;
  isLoading?: boolean;
  isFetching?: boolean;
  error?: unknown;
  refetch: ReturnType<typeof vi.fn>;
};
let readState: ReadState;
const write = vi.fn();

vi.mock("@/app/aos", () => ({
  aos: {
    client: {
      file: {
        read: { useQuery: () => readState },
        write: { useMutation: () => ({ mutate: write, loading: false }) },
      },
    },
    stores: {
      files: { actions: { setDraft: vi.fn(), clearDraft: vi.fn() } },
      viewport: { state: { tabs: { items: [] } }, actions: { updateTab: vi.fn(), createTab: vi.fn(), setActiveTab: vi.fn() } },
    },
  },
}));

// The real viewers are Monaco and friends; a textarea keeps the contract the
// component relies on — show `content`, report edits, honour `readOnly`.
function FakeEditor(props: { content: string; readOnly?: boolean; onChange?: (next: string) => void }) {
  return (
    <textarea
      aria-label="editor"
      value={props.content}
      readOnly={props.readOnly}
      onChange={(event) => {
        if (!props.readOnly) props.onChange?.(event.target.value);
      }}
    />
  );
}
vi.mock("./files-text-viewer.component", () => ({ FilesTextViewer: FakeEditor }));
vi.mock("./files-markdown-viewer.component", () => ({ FilesMarkdownViewer: FakeEditor }));
vi.mock("./files-json-viewer.component", () => ({ FilesJsonViewer: FakeEditor }));
vi.mock("./files-image-viewer.component", () => ({ FilesImageViewer: () => null }));
vi.mock("./files-video-viewer.component", () => ({ FilesVideoViewer: () => null }));
vi.mock("./files-external-viewer.component", () => ({ FilesExternalViewer: () => null }));
vi.mock("./files-pdf-viewer.component", () => ({ FilesPdfViewer: () => null }));
vi.mock("./files-excalidraw-viewer.component", () => ({ FilesExcalidrawViewer: () => null }));

const { FilesContent } = await import("./index");

function readOf(content: string, extra: Record<string, unknown> = {}) {
  return {
    content,
    file: { path: "notes.txt", name: "notes.txt", size: content.length, type: "file" },
    truncated: false,
    binary: false,
    editable: true,
    ...extra,
  };
}

function renderTab() {
  return render(
    <FilesContent
      activeFilePath="notes.txt"
      activeFileTabId="tab-1"
      activeFileTabMetadata={{ filePath: "notes.txt", fileViewer: "text", fileExplorerContext: '{"type":"main"}' }}
    />,
  );
}

const saveButton = () => screen.getByRole("button", { name: "Save" });

beforeEach(() => {
  realtimeHandler = undefined;
  write.mockClear();
  readState = { data: readOf("hello"), refetch: vi.fn() };
});

afterEach(() => cleanup());

describe("FilesContent and the workspace changing underneath it", () => {
  // The daemon's change events carry no explorer context, and the handler
  // compared that `undefined` field — "Cannot read properties of undefined
  // (reading 'type')" on every event while a text file was open, which also
  // stopped delivery to every realtime listener registered after it.
  it("refetches the open file on a change event with no context, instead of throwing", async () => {
    renderTab();
    expect(realtimeHandler).toBeDefined();

    expect(() => realtimeHandler!({ context: undefined, changes: [{ path: "notes.txt" }] })).not.toThrow();
    expect(readState.refetch).toHaveBeenCalledTimes(1);
  });

  it("leaves the file alone when the change is somewhere else", () => {
    renderTab();
    realtimeHandler!({ context: undefined, changes: [{ path: "other.txt" }] });
    expect(readState.refetch).not.toHaveBeenCalled();
  });
});

describe("FilesContent saving", () => {
  it("saves the edited buffer of a whole read", async () => {
    renderTab();
    expect(await screen.findByLabelText("editor")).toHaveProperty("value", "hello");
    expect(saveButton()).toHaveProperty("disabled", true);

    fireEvent.change(screen.getByLabelText("editor"), { target: { value: "hello world" } });
    expect(saveButton()).toHaveProperty("disabled", false);
    fireEvent.click(saveButton());

    expect(write).toHaveBeenCalledWith(
      expect.objectContaining({ body: expect.objectContaining({ path: "notes.txt", content: "hello world" }) }),
    );
  });

  // A truncated read is the beginning of the file; saving it cuts the file.
  it("never saves a truncated read", async () => {
    readState = { data: readOf("the beginning", { truncated: true, editable: false }), refetch: vi.fn() };
    renderTab();
    fireEvent.change(await screen.findByLabelText("editor"), { target: { value: "edited" } });
    expect(saveButton()).toHaveProperty("disabled", true);
    expect(screen.getByText("Too large to edit — showing the beginning")).toBeTruthy();
  });

  // A failed read leaves the buffer empty; saving it would empty the file.
  it("never saves after a failed read, and says why it failed", async () => {
    readState = { data: undefined, isError: true, error: new Error("could not read \"notes.txt\""), refetch: vi.fn() };
    renderTab();
    expect(saveButton()).toHaveProperty("disabled", true);
    expect(screen.getByText("could not read \"notes.txt\"")).toBeTruthy();
  });

  // Bytes that are not text opened as one empty, read-only line with nothing
  // saying why — indistinguishable from an empty file that refuses edits.
  it("says a binary file is binary instead of showing an empty editor", async () => {
    readState = { data: readOf("", { binary: true, editable: false }), refetch: vi.fn() };
    renderTab();
    expect(await screen.findByText("This file is not text")).toBeTruthy();
    expect(screen.queryByLabelText("editor")).toBeNull();
    expect(saveButton()).toHaveProperty("disabled", true);
  });
});
