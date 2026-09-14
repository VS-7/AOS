import { beforeEach, describe, expect, it, vi } from "vitest";

const toast = { success: vi.fn(), error: vi.fn() };
vi.mock("sonner", () => ({ toast }));
vi.mock("@pierre/trees", () => ({ prepareFileTreeInput: (paths: string[]) => ({ prepared: paths }) }));

const move = vi.fn(async (_opts: unknown): Promise<{ data?: unknown; error?: unknown }> => ({ data: {} }));
vi.mock("@/lib/aos-facade", () => ({ api: { file: { move: { mutate: (opts: unknown) => move(opts) } } } }));

const tabs: { items: Array<Record<string, any>> } = { items: [] };
const updateTab = vi.fn();
vi.mock("@/app/aos", () => ({ aos: { stores: { viewport: { state: { tabs }, actions: { updateTab } } } } }));

const { handleTreeMove, retargetOpenTabs, treePathsOf, undoTreeMove } = await import("./files-tree-move.helper");

const main = { type: "main" as const };

beforeEach(() => {
  move.mockClear();
  updateTab.mockClear();
  toast.success.mockClear();
  toast.error.mockClear();
  tabs.items = [];
});

describe("retargetOpenTabs", () => {
  // A tab left on the old path wrote the next save to that path, recreating
  // the file — or the whole folder — the person had just renamed.
  it("follows a moved file and everything under a moved folder, and nothing else", () => {
    tabs.items = [
      { id: "a", type: "file", title: "readme.txt", metadata: { filePath: "docs/readme.txt", fileName: "readme.txt" } },
      { id: "b", type: "file", title: "deep.md", metadata: { filePath: "docs/sub/deep.md" } },
      { id: "c", type: "file", title: "docs-notes.md", metadata: { filePath: "docs-notes.md" } },
      { id: "d", type: "browser", title: "x", metadata: { filePath: "docs/readme.txt" } },
    ];

    retargetOpenTabs("docs/", "documents/");

    expect(updateTab).toHaveBeenCalledTimes(2);
    expect(updateTab).toHaveBeenCalledWith("a", {
      title: "readme.txt",
      metadata: { filePath: "documents/readme.txt", fileName: "readme.txt" },
    });
    expect(updateTab).toHaveBeenCalledWith("b", expect.objectContaining({
      metadata: expect.objectContaining({ filePath: "documents/sub/deep.md" }),
    }));
  });

  it("renames the tab of a renamed file", () => {
    tabs.items = [{ id: "a", type: "file", title: "notes.md", metadata: { filePath: "notes.md" } }];
    retargetOpenTabs("notes.md", "notes2.md");
    expect(updateTab).toHaveBeenCalledWith("a", { title: "notes2.md", metadata: { filePath: "notes2.md", fileName: "notes2.md" } });
  });
});

describe("undoTreeMove", () => {
  const snapshot = { paths: ["docs", "notes.md"], pathIndex: { docs: { type: "directory" }, "notes.md": { type: "file" } } } as never;

  it("moves the tree back when it holds the new name and not the old", () => {
    const present = new Set(["notes2.md"]);
    const model = { getItem: (p: string) => present.has(p), move: vi.fn(), resetPaths: vi.fn() };
    undoTreeMove(model, snapshot, "notes.md", "notes2.md");
    expect(model.move).toHaveBeenCalledWith("notes2.md", "notes.md");
    expect(model.resetPaths).not.toHaveBeenCalled();
  });

  it("redraws the tree from the last snapshot when moving back fails", () => {
    const model = {
      getItem: (p: string) => p === "notes2.md",
      move: vi.fn(() => {
        throw new Error("no such row");
      }),
      resetPaths: vi.fn(),
    };
    undoTreeMove(model, snapshot, "notes.md", "notes2.md");
    expect(model.resetPaths).toHaveBeenCalledWith({ preparedInput: { prepared: ["docs/", "notes.md"] } });
  });

  it("leaves a tree that already has the old name alone", () => {
    const model = { getItem: () => true, move: vi.fn(), resetPaths: vi.fn() };
    undoTreeMove(model, snapshot, "notes.md", "notes2.md");
    expect(model.move).not.toHaveBeenCalled();
  });
});

describe("handleTreeMove", () => {
  it("says a refusal, puts the tree back and rereads it, and moves no tab", async () => {
    move.mockResolvedValueOnce({ error: { message: "\"notes2.md\" already exists." } });
    tabs.items = [{ id: "a", type: "file", metadata: { filePath: "data.json" } }];
    const after = { revert: vi.fn(), refresh: vi.fn() };

    await handleTreeMove("data.json", "notes2.md", main, after);

    expect(move).toHaveBeenCalledWith({ body: { fromPath: "data.json", toPath: "notes2.md", context: main } });
    expect(toast.error).toHaveBeenCalledWith("\"notes2.md\" already exists.");
    expect(after.revert).toHaveBeenCalled();
    expect(after.refresh).toHaveBeenCalled();
    expect(updateTab).not.toHaveBeenCalled();
  });

  it("takes open tabs along when the move lands", async () => {
    tabs.items = [{ id: "a", type: "file", metadata: { filePath: "data.json" } }];
    const after = { revert: vi.fn(), refresh: vi.fn() };

    await handleTreeMove("data.json", "docs/data.json", main, after);

    expect(after.revert).not.toHaveBeenCalled();
    expect(after.refresh).toHaveBeenCalled();
    expect(updateTab).toHaveBeenCalledWith("a", expect.objectContaining({ title: "data.json" }));
    expect(toast.success).toHaveBeenCalledWith("Moved.");
  });

  it("asks nothing for a move onto itself", async () => {
    await handleTreeMove("a.txt", "a.txt", main, { revert: vi.fn(), refresh: vi.fn() });
    expect(move).not.toHaveBeenCalled();
  });
});

describe("treePathsOf", () => {
  it("marks the daemon's directories the way the tree tells them apart", () => {
    expect(treePathsOf({ paths: ["docs", "docs/a.txt", "x/"], pathIndex: { docs: { type: "directory" }, "x/": { type: "directory" } } } as never))
      .toEqual(["docs/", "docs/a.txt", "x/"]);
  });
});
