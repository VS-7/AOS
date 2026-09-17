import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The explorer's writes, through the map every call site goes through.
 *
 * Each of these was wired to the wrong half of a contract: rename, drag and
 * cut sent `fromPath`/`toPath` and the map read `from`/`to`, so every move
 * asked the daemon to move "" to ""; New Folder sent `type: "directory"` and
 * the map wrote a file.
 */
vi.mock("./file", () => ({
  write: vi.fn(async (path: string) => ({ path })),
  create: vi.fn(async (path: string) => ({ path })),
  mkdir: vi.fn(async (path: string) => ({ path })),
  copy: vi.fn(async (_from: string, to: string) => ({ path: to })),
  move: vi.fn(async (_from: string, to: string) => ({ path: to })),
  remove: vi.fn(async (path: string) => ({ path })),
  read: vi.fn(),
  diff: vi.fn(),
  tree: vi.fn(),
  changes: vi.fn(),
}));

const fileApi = await import("./file");
const { COMMAND_MAP } = await import("./command-map");

type Handler = (payload: Record<string, unknown>) => Promise<unknown>;
const handler = (key: string) => COMMAND_MAP[key] as Handler;

beforeEach(() => vi.clearAllMocks());

describe("file.* in the command map", () => {
  it("moves by the names the explorer sends, and by the short ones too", async () => {
    await handler("file.move")({ fromPath: "notes.md", toPath: "notes2.md", context: { type: "main" } });
    expect(fileApi.move).toHaveBeenCalledWith("notes.md", "notes2.md");

    await handler("file.move")({ from: "a.md", to: "b.md" });
    expect(fileApi.move).toHaveBeenLastCalledWith("a.md", "b.md");
  });

  it("creates a directory when asked for one, and a file that refuses to overwrite otherwise", async () => {
    await expect(handler("file.create")({ path: "assets", type: "directory" })).resolves.toEqual({ path: "assets" });
    expect(fileApi.mkdir).toHaveBeenCalledWith("assets");
    expect(fileApi.write).not.toHaveBeenCalled();

    await handler("file.create")({ path: "notes.md", type: "file" });
    expect(fileApi.create).toHaveBeenCalledWith("notes.md", "");
    expect(fileApi.write).not.toHaveBeenCalled();
  });

  it("copies through the daemon", async () => {
    await handler("file.copy")({ fromPath: "pixel.png", toPath: "docs/pixel.png" });
    expect(fileApi.copy).toHaveBeenCalledWith("pixel.png", "docs/pixel.png");
  });

  it("strips the trailing slash the tree puts on a directory before it reaches the daemon", async () => {
    await handler("file.delete")({ path: "docs/" });
    expect(fileApi.remove).toHaveBeenCalledWith("docs");
    await handler("file.move")({ fromPath: "docs/", toPath: "papers/" });
    expect(fileApi.move).toHaveBeenLastCalledWith("docs", "papers");
  });
});
