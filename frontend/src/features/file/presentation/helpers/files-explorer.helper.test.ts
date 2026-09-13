import { describe, expect, it } from "vitest";
import { copyDestinationPath, resolveCreateParentPath } from "./files-explorer.helper";

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
