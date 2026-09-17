import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

vi.mock("@/components/ui/sidebar", async () => (await import("../sidebar-test-doubles")).sidebar);
vi.mock("@/components/ui/dropdown-menu", async () => (await import("../sidebar-test-doubles")).dropdown);
vi.mock("@/components/ui/collapsible", async () => (await import("../sidebar-test-doubles")).collapsible);
vi.mock("@/components/ui/icon", () => ({ Icon: () => null }));
vi.mock("@/features/artifact/presentation/components/create-artifact-dialog", () => ({ CreateArtifactDialog: ({ children }: any) => children }));
let renameDialog: any = null;
vi.mock("./components/rename-surface-dialog", () => ({
  RenameSurfaceDialog: (props: any) => {
    renameDialog = props;
    return null;
  },
}));

const toast = { success: vi.fn(), error: vi.fn() };
vi.mock("sonner", () => ({ toast }));
const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));
const confirm = vi.fn(async () => true);
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm }) }));

const openView = vi.fn();
let views: any[] = [];
vi.mock("@/features/view/presentation/hooks/use-views", () => ({
  useViews: () => ({ views, open: openView, isCurrent: () => false }),
}));
const openArtifact = vi.fn();
let artifacts: any[] = [];
vi.mock("@/features/artifact/presentation/hooks/use-artifacts", () => ({
  useArtifacts: () => ({ artifacts, open: openArtifact, current: undefined }),
}));
const requestArtifactAccess = vi.fn();
vi.mock("@/features/artifact/presentation/helpers/artifact-access", () => ({ requestArtifactAccess: (...a: unknown[]) => requestArtifactAccess(...a) }));

const deleteView = vi.fn(async (_o: unknown) => ({}));
const deleteArtifact = vi.fn(async (_o: unknown) => ({}));
const updateArtifact = vi.fn(async (_o: unknown) => ({}));
const closeTab = vi.fn();
const updateTab = vi.fn();
const tabs = { items: [] as any[] };
vi.mock("@/app/aos", () => ({
  aos: {
    client: {
      view: { delete: { mutateOrThrow: (o: unknown) => deleteView(o) } },
      artifact: {
        delete: { mutateOrThrow: (o: unknown) => deleteArtifact(o) },
        update: { mutateOrThrow: (o: unknown) => updateArtifact(o) },
      },
    },
    stores: {
      viewport: { state: { tabs }, actions: { closeTab, updateTab } },
      view: { actions: { refresh: vi.fn(async () => undefined) } },
      artifact: { actions: { refresh: vi.fn(async () => undefined) } },
    },
  },
}));

const { WorkspaceSidebarSurfacesGroupMenu } = await import("./index");

const row = (label: string) => screen.getByRole("button", { name: label }).parentElement!.parentElement!;

beforeEach(() => {
  views = [
    { id: "contacts-table", name: "Contacts table", title: "Contacts table", scope: "user" },
    { id: "crm-table", name: "CRM table", title: "CRM table", scope: "skill", skill: "crm" },
  ];
  artifacts = [
    { id: "report", name: "Report", visibility: "by_password", hasPassword: false, urls: { local: "/v/artifacts/report/" } },
    { id: "notes", name: "Notes", visibility: "private", urls: { local: "/v/artifacts/notes/" } },
    { id: "plugin-page", name: "Plugin page", visibility: "private", skill: "crm", urls: { local: "/v/artifacts/plugin-page/" } },
  ];
  tabs.items = [];
  renameDialog = null;
  for (const fn of [openView, openArtifact, requestArtifactAccess, deleteView, deleteArtifact, updateArtifact, closeTab, updateTab, navigate, confirm, toast.success, toast.error]) fn.mockClear();
});

afterEach(() => cleanup());

describe("Surfaces rows", () => {
  // A skill's view is found only with its skill; opened by id alone it was
  // "Page not found".
  it("opens a view with its skill", () => {
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    fireEvent.click(screen.getByRole("button", { name: "CRM table" }));
    expect(openView).toHaveBeenCalledWith("crm-table", "crm");
    fireEvent.click(screen.getByRole("button", { name: "Contacts table" }));
    expect(openView).toHaveBeenCalledWith("contacts-table", undefined);
  });

  // What a skill brought leaves with the skill, from the marketplace.
  it("offers no actions on what a skill brought", () => {
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    expect(screen.queryByRole("button", { name: "Actions for CRM table" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Actions for Plugin page" })).toBeNull();
  });

  it("offers Set password only on a by_password artifact, and asks for it there", () => {
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    expect(within(row("Notes")).queryByRole("menuitem", { name: "Set password" })).toBeNull();
    expect(within(row("Contacts table")).queryByRole("menuitem", { name: "Rename" })).toBeNull();

    fireEvent.click(within(row("Report")).getByRole("menuitem", { name: "Set password" }));
    expect(requestArtifactAccess).toHaveBeenCalledWith(expect.objectContaining({ id: "report" }), "set");
  });

  it("deletes a view after asking, by its id", async () => {
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    fireEvent.click(within(row("Contacts table")).getByRole("menuitem", { name: "Delete" }));
    await waitFor(() => expect(deleteView).toHaveBeenCalledWith({ params: { view: "contacts-table" } }));
    expect(confirm).toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith("Deleted.");
  });

  it("deletes an artifact and closes the tabs open on it", async () => {
    tabs.items = [
      { id: "t1", type: "browser", metadata: { artifactId: "notes" } },
      { id: "t2", type: "browser", metadata: { artifactId: "report" } },
    ];
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    fireEvent.click(within(row("Notes")).getByRole("menuitem", { name: "Delete" }));
    await waitFor(() => expect(deleteArtifact).toHaveBeenCalledWith({ params: { artifact: "notes" } }));
    expect(closeTab).toHaveBeenCalledWith("t1");
    expect(closeTab).not.toHaveBeenCalledWith("t2");
  });

  // The sidebar row took the new name, the tab open on the artifact kept the
  // old one and there was no way to refresh it.
  it("renames an artifact and retitles the tab open on it", async () => {
    tabs.items = [
      { id: "t1", type: "browser", metadata: { artifactId: "notes" } },
      { id: "t2", type: "browser", metadata: { artifactId: "report" } },
    ];
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    fireEvent.click(within(row("Notes")).getByRole("menuitem", { name: "Rename" }));
    await renameDialog.onRename("Meeting notes");
    expect(updateArtifact).toHaveBeenCalledWith({ params: { artifact: "notes" }, body: { name: "Meeting notes" } });
    expect(updateTab).toHaveBeenCalledWith("t1", { title: "Meeting notes" });
    expect(updateTab).not.toHaveBeenCalledWith("t2", expect.anything());
    expect(toast.success).toHaveBeenCalledWith("Renamed.");
  });

  it("says why a delete was refused, and keeps the row", async () => {
    deleteView.mockRejectedValueOnce(new Error("view is in use"));
    render(<WorkspaceSidebarSurfacesGroupMenu />);
    fireEvent.click(within(row("Contacts table")).getByRole("menuitem", { name: "Delete" }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("view is in use"));
    expect(toast.success).not.toHaveBeenCalled();
  });
});
