import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The Changes panel crashed with `undefined is not an object (evaluating
 * 'm.summary.fileCount')` the moment its query resolved: this adapter built
 * the snapshot without the `summary` the panel's header reads, and
 * `changes-content.tsx` guarded only the snapshot itself
 * (`snapshot?.summary.fileCount`), so a resolved-but-summaryless snapshot
 * threw on the property access rather than falling back to zero.
 *
 * The summary is part of the contract the panel was ported against — the
 * original server answers `{ fileCount, additions, deletions }` beside the
 * file list — so it belongs here, computed from the files, and not patched
 * over at the one call site that happened to read it.
 */
vi.mock("./file", () => ({
  tree: vi.fn(),
  changes: vi.fn(),
  read: vi.fn(),
  diff: vi.fn(),
}));

vi.mock("./client", () => ({
  client: { invoke: vi.fn() },
}));

const { tree, changes: rawChanges, read: rawRead, diff: rawDiff } = await import("./file");
const { client } = await import("./client");
const { changes, explorer, readForEditor, diffForPanel, listTasks } = await import("./file-explorer");

const mockTree = vi.mocked(tree);
const mockRawChanges = vi.mocked(rawChanges);
const mockRead = vi.mocked(rawRead);
const mockDiff = vi.mocked(rawDiff);
const mockInvoke = vi.mocked(client.invoke);

beforeEach(() => {
  vi.clearAllMocks();
  mockTree.mockResolvedValue({ path: "", nodes: [] });
  mockInvoke.mockResolvedValue({ tasks: [] });
});

describe("changes()", () => {
  it("carries the summary the Changes header reads", async () => {
    mockRawChanges.mockResolvedValue({
      files: [
        { path: "a.ts", status: "modified" },
        { path: "b.ts", status: "added" },
        { path: "c.ts", status: "deleted" },
      ],
      total: 3,
    });

    const { snapshot } = await changes();

    expect(snapshot.summary).toEqual({
      fileCount: 3,
      additions: 0,
      deletions: 0,
    });
  });

  it("sums the per-file counts when the daemon reports them", async () => {
    mockRawChanges.mockResolvedValue({
      files: [
        { path: "a.ts", status: "modified", additions: 4, deletions: 1 },
        { path: "b.ts", status: "added", additions: 9 },
      ] as never,
      total: 2,
    });

    const { snapshot } = await changes();

    expect(snapshot.summary).toEqual({
      fileCount: 2,
      additions: 13,
      deletions: 1,
    });
  });

  it("answers a zeroed summary on a clean tree rather than nothing", async () => {
    mockRawChanges.mockResolvedValue({ files: [], total: 0 });

    const { snapshot } = await changes();

    expect(snapshot.summary).toEqual({
      fileCount: 0,
      additions: 0,
      deletions: 0,
    });
  });
});

describe("explorer()", () => {
  it("carries a summary too, so one snapshot type means one shape", async () => {
    mockTree.mockResolvedValue({
      path: "",
      nodes: [
        {
          path: "a.ts",
          name: "a.ts",
          dir: false,
          size: 10,
          editable: true,
          modifiedAt: "2026-01-01T00:00:00Z",
        },
      ],
    });
    mockRawChanges.mockResolvedValue({
      files: [{ path: "a.ts", status: "modified" }],
      total: 1,
    });

    const { snapshot } = await explorer({ includeContexts: false });

    expect(snapshot.summary).toEqual({
      fileCount: 1,
      additions: 0,
      deletions: 0,
    });
  });

  // The daemon going down used to read as "No files available — This
  // explorer context is empty right now": the tree's failure was swallowed
  // into an empty snapshot, so the explorer's own error branch ("Unable to
  // load files") could never render, and React Query had nothing to retry —
  // the empty answer stayed on screen after the daemon came back.
  it("fails when the tree cannot be read, instead of answering an empty workspace", async () => {
    mockTree.mockRejectedValue(Object.assign(new Error("the daemon is not answering"), { code: "AOS_DAEMON_UNREACHABLE" }));
    mockRawChanges.mockResolvedValue({ files: [], total: 0 });

    await expect(explorer({ includeContexts: false })).rejects.toMatchObject({
      code: "AOS_DAEMON_UNREACHABLE",
    });
  });

  it("still answers a summary when the changes call fails outright", async () => {
    mockRawChanges.mockRejectedValue(new Error("not a git repository"));

    const { snapshot } = await explorer({ includeContexts: false });

    expect(snapshot.summary).toEqual({
      fileCount: 0,
      additions: 0,
      deletions: 0,
    });
    expect(snapshot.files).toEqual([]);
  });
});

const node = (path: string, dir = false) => ({
  path,
  name: path.split("/").pop()!,
  dir,
  size: dir ? 0 : 12,
  editable: !dir,
  modifiedAt: "2026-01-01T00:00:00Z",
  ...(dir ? {} : { extension: path.split(".").pop() }),
});

describe("explorer() pathIndex", () => {
  // The tree listed every file and opened none of them: the index held
  // {type, name, size, editable} and no path, the click handler handed that
  // entry to the tab builder, and every tab it made said "No file selected" —
  // then matched every later click, because undefined === undefined.
  it("indexes every node as a file the tab builder can open", async () => {
    mockTree.mockResolvedValue({
      path: "",
      nodes: [node("docs", true), node("docs/readme.txt"), node("notes.md"), node("data.json"), node("pixel.png")],
    });
    mockRawChanges.mockResolvedValue({ files: [], total: 0 });

    const { snapshot } = await explorer({ includeContexts: false });

    expect(snapshot.pathIndex["notes.md"]).toMatchObject({ path: "notes.md", type: "file", viewer: "markdown" });
    expect(snapshot.pathIndex["data.json"]).toMatchObject({ path: "data.json", viewer: "json" });
    expect(snapshot.pathIndex["docs/readme.txt"]).toMatchObject({ path: "docs/readme.txt", viewer: "text", parentPath: "docs" });
    expect(snapshot.pathIndex["pixel.png"]).toMatchObject({ path: "pixel.png", viewer: "image" });
    expect(snapshot.pathIndex["docs"]).toMatchObject({ path: "docs", type: "directory" });
  });

  // The context switcher is hidden until the daemon can resolve a worktree,
  // so nothing asks for the task list it would have shown.
  it("does not list tasks unless asked", async () => {
    mockRawChanges.mockResolvedValue({ files: [], total: 0 });
    await explorer();
    expect(mockInvoke).not.toHaveBeenCalled();
  });
});

describe("listTasks()", () => {
  // The picker showed raw UUIDs: Go tasks carry `name`, never `title`, and it
  // offered every task rather than the ones that have a worktree to switch to.
  it("labels tasks by name and offers only those with a worktree", async () => {
    mockInvoke.mockResolvedValue({
      tasks: [
        { id: "t-1", name: "Build the API", worktree: { enabled: true } },
        { id: "t-2", name: "No checkout" },
      ],
    });
    await expect(listTasks()).resolves.toEqual([{ id: "t-1", title: "Build the API" }]);
  });
});

describe("readForEditor()", () => {
  // The editor read `content` and `file` off an answer that has `text`, so
  // every file opened empty — and Save wrote that emptiness over the file.
  it("answers the text as content, with the file it came from", async () => {
    mockRead.mockResolvedValue({ path: "docs/readme.txt", mediaType: "text/plain", text: "hello docs\n", size: 11, truncated: false });

    const answer = await readForEditor("docs/readme.txt");

    expect(answer.content).toBe("hello docs\n");
    expect(answer.editable).toBe(true);
    expect(answer.file).toMatchObject({ path: "docs/readme.txt", name: "readme.txt", size: 11, viewer: "text" });
  });

  it("marks a truncated or binary read as not editable, so saving it cannot cut the file short", async () => {
    mockRead.mockResolvedValue({ path: "big.log", mediaType: "text/plain", text: "part", size: 99999999, truncated: true });
    expect((await readForEditor("big.log")).editable).toBe(false);

    mockRead.mockResolvedValue({ path: "blob", mediaType: "application/octet-stream", base64: "AAE=", size: 2, truncated: false });
    const binary = await readForEditor("blob");
    expect(binary.editable).toBe(false);
    expect(binary.content).toBe("");
    // Said so, so the editor can say it rather than show an empty buffer.
    expect(binary.binary).toBe(true);
  });

  it("keeps an empty file editable — empty is content, not a failure", async () => {
    mockRead.mockResolvedValue({ path: "empty.md", mediaType: "text/markdown", size: 0, truncated: false });
    const answer = await readForEditor("empty.md");
    expect(answer.content).toBe("");
    expect(answer.editable).toBe(true);
    expect(answer.binary).toBe(false);
  });
});

describe("diffForPanel()", () => {
  // The Changes panel read `snapshot.{oldFile,newFile}` off a diff that
  // answers `oldText`/`newText`, so every diff said "not available".
  it("answers both sides as the files the diff viewer renders", async () => {
    mockDiff.mockResolvedValue({ path: "notes.md", status: "modified", isBinary: false, oldText: "a\n", newText: "b\n" });

    const { snapshot } = await diffForPanel("notes.md");

    expect(snapshot).toEqual({
      status: "modified",
      isBinary: false,
      oldFile: { name: "notes.md", contents: "a\n" },
      newFile: { name: "notes.md", contents: "b\n" },
    });
  });

  it("leaves out the side a file does not have", async () => {
    mockDiff.mockResolvedValue({ path: "new.md", status: "untracked", isBinary: false, newText: "hi" });
    const { snapshot } = await diffForPanel("new.md");
    expect(snapshot.oldFile).toBeUndefined();
    expect(snapshot.newFile).toEqual({ name: "new.md", contents: "hi" });
  });
});
