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
}));

vi.mock("./client", () => ({
  client: { invoke: vi.fn() },
}));

const { tree, changes: rawChanges } = await import("./file");
const { client } = await import("./client");
const { changes, explorer } = await import("./file-explorer");

const mockTree = vi.mocked(tree);
const mockRawChanges = vi.mocked(rawChanges);
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
