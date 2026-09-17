import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

vi.mock("@/components/ui/sidebar", async () => (await import("../sidebar-test-doubles")).sidebar);
vi.mock("@/components/ui/dropdown-menu", async () => (await import("../sidebar-test-doubles")).dropdown);
vi.mock("@/components/ui/collapsible", async () => (await import("../sidebar-test-doubles")).collapsible);
vi.mock("./components/create-collection-dialog", () => ({ CreateCollectionDialog: () => null }));

const toast = { success: vi.fn(), error: vi.fn() };
vi.mock("sonner", () => ({ toast }));
const navigate = vi.fn();
let pathname = "/";
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
  useRouterState: ({ select }: any) => select({ location: { pathname } }),
}));
const confirm = vi.fn(async () => true);
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm }) }));

let items: any[] = [];
const deleteCollection = vi.fn(async (_o: unknown) => ({}));
const refresh = vi.fn(async () => undefined);
vi.mock("@/app/aos", () => ({
  aos: {
    client: { collection: { delete: { mutateOrThrow: (o: unknown) => deleteCollection(o) } } },
    stores: { collections: { useState: (select: any) => select({ items }), actions: { refresh } } },
  },
}));

const { WorkspaceSidebarCollectionsGroupMenu } = await import("./index");

const row = (label: string) => screen.getByRole("button", { name: label }).parentElement!;

beforeEach(() => {
  items = [
    { id: "contacts", name: "Contacts", scope: "workspace" },
    { id: "crm-leads", name: "CRM leads", scope: "skill", skill: "crm" },
  ];
  pathname = "/";
  for (const fn of [navigate, confirm, deleteCollection, refresh, toast.success, toast.error]) fn.mockClear();
});

afterEach(() => cleanup());

describe("Collections rows", () => {
  // By id — the collection's directory — never by its name.
  it("opens a collection by its id", () => {
    render(<WorkspaceSidebarCollectionsGroupMenu />);
    fireEvent.click(screen.getByRole("button", { name: "CRM leads" }));
    expect(navigate).toHaveBeenCalledWith({ to: "/collections/$id", params: { id: "crm-leads" } });
  });

  it("offers no Delete on a skill's collection", () => {
    render(<WorkspaceSidebarCollectionsGroupMenu />);
    expect(within(row("CRM leads")).queryByRole("menuitem", { name: "Delete" })).toBeNull();
    expect(within(row("Contacts")).getByRole("menuitem", { name: "Delete" })).toBeTruthy();
  });

  it("deletes after asking, and leaves the page of the collection it removed", async () => {
    pathname = "/collections/contacts";
    render(<WorkspaceSidebarCollectionsGroupMenu />);
    expect(screen.getByRole("button", { name: "Contacts" }).getAttribute("data-active")).toBe("true");

    fireEvent.click(within(row("Contacts")).getByRole("menuitem", { name: "Delete" }));
    await waitFor(() => expect(deleteCollection).toHaveBeenCalledWith({ params: { collection: "contacts" } }));
    expect(refresh).toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith({ to: "/" });
    expect(toast.success).toHaveBeenCalledWith("Deleted.");
  });

  it("says why a delete was refused", async () => {
    deleteCollection.mockRejectedValueOnce(new Error("a view still shows it"));
    render(<WorkspaceSidebarCollectionsGroupMenu />);
    fireEvent.click(within(row("Contacts")).getByRole("menuitem", { name: "Delete" }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("a view still shows it"));
    expect(navigate).not.toHaveBeenCalled();
  });
});
