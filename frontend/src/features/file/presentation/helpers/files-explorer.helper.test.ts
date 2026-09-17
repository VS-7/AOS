import { describe, expect, it } from "vitest";
import { copyDestinationPath, explorerContextsEqual, isDirectoryEntry, resolveCreateParentPath } from "./files-explorer.helper";

describe("resolveCreateParentPath", () => {
  // @pierre/trees focuses its first row on load. Reading that focus as the
  // person's choice put every header "New file" inside whichever folder
  // sorted first — "Create inside docs." with nothing selected.
  it("creates at the root when nothing is selected, whatever row has focus", () => {
    expect(resolveCreateParentPath({ focusedPath: "docs/", selectedPaths: [] })).toBe("");
  });

  it("creates inside a selected folder, and beside a selected file", () => {
    expect(resolveCreateParentPath({ selectedPaths: ["docs/"] })).toBe("docs");
    expect(resolveCreateParentPath({ selectedPaths: ["docs/readme.txt"] })).toBe("docs");
    expect(
      resolveCreateParentPath({
        selectedPaths: ["src"],
        pathIndex: { src: { type: "directory" } },
      }),
    ).toBe("src");
  });
});

describe("copyDestinationPath", () => {
  // A copy pasted into its own folder used to be written over the original.
  it("keeps the name when nothing is there", () => {
    expect(copyDestinationPath("docs/data.json", new Set(["data.json"]))).toBe("docs/data.json");
  });

  it("names the copy beside what it would have replaced", () => {
    const taken = new Set([".env.sample", "notes.md", "notes copy.md", "docs"]);
    expect(copyDestinationPath(".env.sample", taken)).toBe(".env copy.sample");
    expect(copyDestinationPath("notes.md", taken)).toBe("notes copy 2.md");
    expect(copyDestinationPath("docs", taken)).toBe("docs copy");
    expect(copyDestinationPath("docs/", new Set(["docs/"]))).toBe("docs copy");
  });
});

describe("explorerContextsEqual", () => {
  // The daemon's change events name no context at all; the file API serves
  // only the main workspace. Comparing that missing context threw, and took
  // every open text file's realtime handler down with it.
  it("reads a missing context as the main workspace", () => {
    expect(explorerContextsEqual(undefined, { type: "main" })).toBe(true);
    expect(explorerContextsEqual({ type: "main" }, undefined)).toBe(true);
    expect(explorerContextsEqual(undefined, { type: "task", taskId: "t1" })).toBe(false);
  });

  it("still tells tasks and branches apart", () => {
    expect(explorerContextsEqual({ type: "task", taskId: "a" }, { type: "task", taskId: "a" })).toBe(true);
    expect(explorerContextsEqual({ type: "task", taskId: "a" }, { type: "task", taskId: "b" })).toBe(false);
    expect(explorerContextsEqual({ type: "branch", branch: "x" }, { type: "main" })).toBe(false);
  });
});

describe("isDirectoryEntry", () => {
  // git lists an untracked folder as one entry, its path ending in "/"; there
  // is no file to diff, and asking for one answered an I/O error.
  it("recognises the folder entries git reports", () => {
    expect(isDirectoryEntry("assets/")).toBe(true);
    expect(isDirectoryEntry(".aos/collections/contacts/")).toBe(true);
    expect(isDirectoryEntry("assets/a.txt")).toBe(false);
  });
});
