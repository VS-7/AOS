import { beforeEach, describe, expect, it, vi } from "vitest";

const tabs: { items: Array<Record<string, any>> } = { items: [] };
const createTab = vi.fn();
const setActiveTab = vi.fn();
vi.mock("@/app/aos", () => ({
  aos: { stores: { viewport: { state: { tabs }, actions: { createTab, setActiveTab } } } },
}));

const { ArtifactHelper } = await import("./artifact.helper");
const { currentArtifactAccess, closeArtifactAccess } = await import("./artifact-access");

const artifact = (visibility: string, extra: Record<string, unknown> = {}) =>
  ({
    id: "report",
    name: "Report",
    entrypoint: "index.html",
    visibility,
    createdAt: "",
    updatedAt: "",
    urls: { local: "/v/artifacts/report/", tunnel: null },
    ...extra,
  }) as never;

beforeEach(() => {
  tabs.items = [];
  createTab.mockClear();
  setActiveTab.mockClear();
  closeArtifactAccess();
});

describe("ArtifactHelper.openInBrowserTab", () => {
  it("opens a private or workspace artifact at its address", () => {
    ArtifactHelper.openInBrowserTab(artifact("workspace"));
    expect(createTab).toHaveBeenCalledWith(expect.objectContaining({ type: "browser", url: "/v/artifacts/report/" }));
  });

  // A by_password artifact asks everybody for its password, its creator
  // included. Opened at its bare address, the tab showed the daemon's
  // refusal as raw JSON.
  it("asks for the password of a by_password artifact instead of opening a refusal", () => {
    ArtifactHelper.openInBrowserTab(artifact("by_password", { hasPassword: true }));
    expect(createTab).not.toHaveBeenCalled();
    expect(currentArtifactAccess()).toMatchObject({ artifact: { id: "report" }, mode: "open" });
  });

  it("opens a by_password artifact with the password it is given", () => {
    ArtifactHelper.openInBrowserTab(artifact("by_password", { hasPassword: true }), { password: "open sesame&42" });
    expect(createTab).toHaveBeenCalledWith(
      expect.objectContaining({ url: "/v/artifacts/report/?password=open%20sesame%2642" }),
    );
    expect(currentArtifactAccess()).toBeNull();
  });

  it("brings an artifact's open tab forward rather than asking again", () => {
    tabs.items = [{ id: "t1", type: "browser", metadata: { artifactId: "report" } }];
    ArtifactHelper.openInBrowserTab(artifact("by_password", { hasPassword: true }));
    expect(setActiveTab).toHaveBeenCalledWith("t1");
    expect(currentArtifactAccess()).toBeNull();
  });
});
