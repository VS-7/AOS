import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const mutate = vi.fn(async (_opts: unknown): Promise<{ data?: unknown; error?: unknown }> => ({ data: {}, error: undefined }));
vi.mock("@/lib/aos-facade", () => ({
  api: {
    file: {
      copy: { mutate: (opts: unknown) => mutate({ action: "copy", ...(opts as object) }) },
      move: { mutate: (opts: unknown) => mutate({ action: "move", ...(opts as object) }) },
      read: { query: vi.fn() },
      create: { mutate: vi.fn() },
      list: { query: vi.fn() },
    },
  },
}));

const { pasteFilesClipboard } = await import("./files-tree-clipboard.helper");

const main = { type: "main" as const };
const index = {
  "data.json": { type: "file" as const },
  ".env.sample": { type: "file" as const },
  docs: { type: "directory" as const },
  "docs/readme.txt": { type: "file" as const },
};

beforeEach(() => mutate.mockClear());

describe("pasteFilesClipboard", () => {
  // Paste used to read the file through the JSON API, write back a field the
  // answer did not have, and so create every copy empty; pasting into the
  // same folder wrote that emptiness over the original.
  it("copies through the daemon, under a name nothing is using", async () => {
    await pasteFilesClipboard({
      clipboard: { mode: "copy", paths: ["data.json", ".env.sample"], context: main },
      targetParentPath: "",
      explorerContext: main,
      pathIndex: index,
    });

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ action: "copy", body: expect.objectContaining({ fromPath: "data.json", toPath: "data copy.json" }) }),
    );
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ action: "copy", body: expect.objectContaining({ fromPath: ".env.sample", toPath: ".env copy.sample" }) }),
    );
  });

  it("copies a folder as one request, into another folder under its own name", async () => {
    await pasteFilesClipboard({
      clipboard: { mode: "copy", paths: ["docs/"], context: main },
      targetParentPath: "archive",
      explorerContext: main,
      pathIndex: index,
    });
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ body: expect.objectContaining({ fromPath: "docs", toPath: "archive/docs" }) }),
    );
  });

  it("moves a cut item, and does nothing when it is pasted where it already is", async () => {
    await pasteFilesClipboard({
      clipboard: { mode: "cut", paths: ["data.json"], context: main },
      targetParentPath: "docs",
      explorerContext: main,
      pathIndex: index,
    });
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ action: "move", body: expect.objectContaining({ fromPath: "data.json", toPath: "docs/data.json" }) }),
    );

    mutate.mockClear();
    await pasteFilesClipboard({
      clipboard: { mode: "cut", paths: ["docs/readme.txt"], context: main },
      targetParentPath: "docs",
      explorerContext: main,
      pathIndex: index,
    });
    expect(mutate).not.toHaveBeenCalled();
  });

  it("stops at the first refusal and says what the daemon said", async () => {
    mutate.mockResolvedValueOnce({ data: undefined, error: { message: '"docs" cannot be copied into "docs/docs", which is inside it' } });
    await expect(
      pasteFilesClipboard({
        clipboard: { mode: "copy", paths: ["docs/"], context: main },
        targetParentPath: "docs",
        explorerContext: main,
        pathIndex: index,
      }),
    ).rejects.toThrow("which is inside it");
  });
});
