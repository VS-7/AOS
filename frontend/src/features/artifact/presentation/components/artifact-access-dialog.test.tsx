import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const createTab = vi.fn();
const updateTab = vi.fn();
const openTabs: { items: Array<Record<string, unknown>> } = { items: [] };
const setPassword = vi.fn(async (_opts: unknown) => ({ url: "/v/artifacts/report/" }));
const refresh = vi.fn(async () => undefined);
vi.mock("@/app/aos", () => ({
  aos: {
    client: { artifact: { setPassword: { mutateOrThrow: (opts: unknown) => setPassword(opts) } } },
    stores: {
      viewport: { state: { tabs: openTabs }, actions: { createTab, updateTab, setActiveTab: vi.fn() } },
      artifact: { actions: { refresh } },
    },
  },
}));
const toastSuccess = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (...a: unknown[]) => toastSuccess(...a), error: vi.fn() } }));

const { ArtifactAccessDialog } = await import("./artifact-access-dialog");
const { requestArtifactAccess, currentArtifactAccess } = await import("../helpers/artifact-access");

const artifact = (extra: Record<string, unknown>) =>
  ({
    id: "report",
    name: "Report",
    entrypoint: "index.html",
    visibility: "by_password",
    createdAt: "",
    updatedAt: "",
    urls: { local: "/v/artifacts/report/", tunnel: null },
    ...extra,
  }) as never;

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
  createTab.mockClear();
  updateTab.mockClear();
  openTabs.items = [];
  setPassword.mockClear();
  toastSuccess.mockClear();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function ask(extra: Record<string, unknown>, mode: "open" | "set" = "open") {
  render(<ArtifactAccessDialog />);
  act(() => requestArtifactAccess(artifact(extra), mode));
}

describe("ArtifactAccessDialog", () => {
  it("opens the artifact once the daemon accepts the password", async () => {
    fetchMock.mockResolvedValue(new Response("<html></html>", { status: 200 }));
    ask({ hasPassword: true });

    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "open-sesame-42" } });
    fireEvent.click(screen.getByRole("button", { name: "Open" }));

    await waitFor(() => expect(createTab).toHaveBeenCalled());
    expect(fetchMock).toHaveBeenCalledWith("/v/artifacts/report/?password=open-sesame-42", expect.anything());
    expect(createTab).toHaveBeenCalledWith(expect.objectContaining({ url: "/v/artifacts/report/?password=open-sesame-42" }));
    await waitFor(() => expect(currentArtifactAccess()).toBeNull());
  });

  // Checked before a tab opens, so a wrong password is said in the dialog
  // rather than as the daemon's JSON in a tab.
  it("says the password is wrong, and opens nothing", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "AOS_ARTIFACT_UNAUTHORIZED", message: "not authorized" } }), { status: 403 }),
    );
    ask({ hasPassword: true });

    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "wrong-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Open" }));

    expect(await screen.findByText("That password is not the one this artifact is shared with.")).toBeTruthy();
    expect(createTab).not.toHaveBeenCalled();
  });

  // One an agent created as by_password without a password refuses everybody,
  // and nothing in the interface could give it one.
  it("sets the first password of an artifact that has none, then opens it", async () => {
    ask({ hasPassword: false });
    expect(screen.getByText("“Report” has no password yet")).toBeTruthy();

    const open = screen.getByRole("button", { name: "Set password and open" });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "short" } });
    expect(open).toHaveProperty("disabled", true);
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "long-enough-1" } });
    fireEvent.click(open);

    await waitFor(() => expect(createTab).toHaveBeenCalled());
    expect(setPassword).toHaveBeenCalledWith({ params: { artifact: "report" }, body: { password: "long-enough-1" } });
    expect(refresh).toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(createTab).toHaveBeenCalledWith(expect.objectContaining({ url: "/v/artifacts/report/?password=long-enough-1" }));
  });

  // The tab already open on it carries the old password in its address, and
  // would show the daemon's refusal the next time it loads.
  it("changes the password from the row menu, and points its open tab at the new one", async () => {
    openTabs.items = [
      { id: "t1", type: "browser", url: "/v/artifacts/report/?password=old-password", metadata: { artifactId: "report" } },
      { id: "t2", type: "browser", url: "/v/artifacts/other/", metadata: { artifactId: "other" } },
    ];
    ask({ hasPassword: true }, "set");
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "a-new-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Set password" }));

    await waitFor(() => expect(setPassword).toHaveBeenCalled());
    expect(toastSuccess).toHaveBeenCalledWith("Password set.");
    expect(createTab).not.toHaveBeenCalled();
    expect(updateTab).toHaveBeenCalledTimes(1);
    expect(updateTab).toHaveBeenCalledWith("t1", { url: "/v/artifacts/report/?password=a-new-password" });
  });
});
