import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

// The diff itself is not what these tests are about: only whether it is asked
// for at all.
vi.mock("./changes-file-diff", () => ({ ChangesFileDiff: ({ path }: { path: string }) => <div>diff of {path}</div> }));

const { ChangesFileItem } = await import("./changes-file-item");

const preferences = { diffStyle: "unified", wordWrap: false, ignoreWhitespace: false } as never;

function renderItem(path: string, status = "untracked") {
  return render(
    <ChangesFileItem
      file={{ path, status } as never}
      explorerContext={{ type: "main" }}
      preferences={preferences}
      themeType="light"
      expanded
      onToggle={() => {}}
    />,
  );
}

afterEach(() => cleanup());

describe("ChangesFileItem", () => {
  // git lists an untracked folder as one entry ("assets/"), and every AOS
  // workspace has some under .aos/. Expanding one asked for the diff of a
  // folder and printed the daemon's 'could not read "assets/"'.
  it("expands an untracked folder into a note, never a diff", () => {
    renderItem("assets/");
    expect(screen.queryByText("diff of assets/")).toBeNull();
    expect(screen.getByText("A new folder. Its files are not tracked yet; open them from Files.")).toBeTruthy();
  });

  it("expands a file into its diff", () => {
    renderItem("assets/a.txt", "modified");
    expect(screen.getByText("diff of assets/a.txt")).toBeTruthy();
  });
});
