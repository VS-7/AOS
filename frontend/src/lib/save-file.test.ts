import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const bridge = vi.hoisted(() => ({
  byName: vi.fn(async (_name: string, ..._args: unknown[]) => "" as unknown),
}));

vi.mock("@wailsio/runtime", () => ({
  Call: { ByName: bridge.byName },
  Browser: { OpenURL: vi.fn() },
  Clipboard: { SetText: vi.fn() },
  Dialogs: { Question: vi.fn() },
  System: { IsMac: () => false, IsWindows: () => false, IsLinux: () => false },
  Window: {},
}));

async function loadAt(search: string) {
  window.history.replaceState({}, "", `/${search}`);
  vi.resetModules();
  return import("./save-file");
}

const DESKTOP = "?daemon=http%3A%2F%2F127.0.0.1%3A5326&platform=darwin";

beforeEach(() => {
  bridge.byName.mockReset();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("saving a file the interface produced", () => {
  /**
   * The defect: seven places built a Blob and handed it to `<a download>`,
   * which is what an Electron renderer supports. Wails implements no download
   * delegate on any platform, so the click was accepted and nothing was
   * written — and one of the seven then showed a success toast for a file
   * that never existed.
   */
  it("goes over the bridge in the desktop window, not through an anchor", async () => {
    const save = await loadAt(DESKTOP);
    bridge.byName.mockResolvedValueOnce("/Users/someone/Desktop/export.csv");
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click");

    const result = await save.saveText("a,b\n1,2\n", "export.csv", "text/csv");

    expect(click).not.toHaveBeenCalled();
    expect(result).toEqual({ status: "saved", path: "/Users/someone/Desktop/export.csv" });

    const [name, filename, content] = bridge.byName.mock.calls[0];
    expect(name).toContain("SystemService.SaveFile");
    expect(filename).toBe("export.csv");
    expect(atob(content as string)).toBe("a,b\n1,2\n");
    click.mockRestore();
  });

  it("reads an empty path as a cancelled save panel, not a failure", async () => {
    const save = await loadAt(DESKTOP);
    bridge.byName.mockResolvedValueOnce("");

    expect(await save.saveText("x", "export.csv")).toEqual({ status: "cancelled" });
  });

  it("reports a refusal from the host instead of throwing out of a click handler", async () => {
    const save = await loadAt(DESKTOP);
    bridge.byName.mockRejectedValueOnce(new Error("the chosen location could not be written to"));

    expect(await save.saveText("x", "export.csv")).toEqual({
      status: "failed",
      reason: "the chosen location could not be written to",
    });
  });

  it("keeps the browser's own download in a browser, where it works", async () => {
    const save = await loadAt("");
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    const result = await save.saveText("x", "notes.txt");

    expect(bridge.byName).not.toHaveBeenCalled();
    expect(click).toHaveBeenCalledOnce();
    expect(result).toEqual({ status: "saved", path: "notes.txt" });
    click.mockRestore();
  });

  it("encodes bytes the bridge can carry, in chunks a call can take", async () => {
    const save = await loadAt(DESKTOP);
    bridge.byName.mockResolvedValueOnce("/tmp/big.bin");

    // Larger than the 0x8000 chunk the encoder steps in — spreading an array
    // this size into String.fromCharCode overflows the engine's argument
    // limit, which is why it is chunked at all.
    const bytes = new Uint8Array(0x8000 * 2 + 7).fill(0xab);
    await save.saveBlob(new Blob([bytes]), "big.bin");

    const encoded = bridge.byName.mock.calls[0][2] as string;
    const decoded = atob(encoded);
    expect(decoded.length).toBe(bytes.length);
    expect(decoded.charCodeAt(0)).toBe(0xab);
    expect(decoded.charCodeAt(decoded.length - 1)).toBe(0xab);
  });
});
