import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";

import type { InstalledSkillRecord } from "@/features/marketplace/interfaces/marketplace.interfaces";

/**
 * The page and its address, round trip: `navigate` here is a router that
 * applies the search updater and hands the result back as `search`, the way
 * the route does — a little later, as a real navigation lands.
 */
const router = vi.hoisted(() => ({ navigate: (_: unknown) => {} }));

vi.mock("@tanstack/react-router", async (original) => ({
  ...(await original<object>()),
  useNavigate: () => router.navigate,
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}));

vi.mock("hashvatar/react", () => ({ Hashvatar: () => null }));

const { MarketplacePageInner } = await import("./inner");

type Search = { category?: string; query?: string };

function Page({ initial, installed = [] }: { initial: Search; installed?: InstalledSkillRecord[] }) {
  const [search, setSearch] = React.useState<Search>(initial);
  router.navigate = ({ search: next }: any) => {
    window.setTimeout(() => setSearch((previous) => (typeof next === "function" ? next(previous) : next)), 20);
  };
  return (
    <>
      <output data-testid="address">{search.query ?? ""}</output>
      <MarketplacePageInner
        marketplacePlugins={[]}
        installedPlugins={installed}
        marketplaceError={{ code: "AOS_MARKETPLACE_NO_REGISTRY", message: "No registry is configured." }}
        installedError={null}
        search={search}
      />
    </>
  );
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

async function typeSlowly(box: HTMLInputElement, text: string, gap: number) {
  for (const character of text) {
    fireEvent.change(box, { target: { value: box.value + character } });
    await act(async () => {
      vi.advanceTimersByTime(gap);
    });
  }
}

describe("the marketplace search box", () => {
  // W3-14: the box debounced its value up to the page, which fed it back
  // down as the box's new value — trimmed, and a beat late. Typing at 300ms a
  // key ended as "dm cmpui", and the address followed.
  it("keeps every character typed, however slowly", async () => {
    render(<Page initial={{}} />);
    const box = screen.getByPlaceholderText("Search plugins...") as HTMLInputElement;

    await typeSlowly(box, "demo crm plugin", 300);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(box.value).toBe("demo crm plugin");
    expect(screen.getByTestId("address").textContent).toBe("demo crm plugin");
  });

  it("keeps a space typed at the end of a word", async () => {
    render(<Page initial={{}} />);
    const box = screen.getByPlaceholderText("Search plugins...") as HTMLInputElement;

    await typeSlowly(box, "demo ", 50);
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(box.value).toBe("demo ");
    expect(screen.getByTestId("address").textContent).toBe("demo");
  });

  it("empties when the address is cleared elsewhere", async () => {
    render(<Page initial={{ query: "crm" }} />);
    const box = screen.getByPlaceholderText("Search plugins...") as HTMLInputElement;
    expect(box.value).toBe("crm");

    fireEvent.click(screen.getByText("Clear filters"));
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(box.value).toBe("");
    expect(screen.getByTestId("address").textContent).toBe("");
  });
});

describe("the marketplace with no registry", () => {
  const demoCrm = { id: "demo-crm", name: "Demo CRM", description: "Contacts and deals." } as InstalledSkillRecord;

  // With no registry the page draws only what is installed, and it drew all
  // of it under ?category=Development — "1 result in Development" over Demo
  // CRM, whose category is Other.
  it("counts and draws only the installed plugins in the chosen category", () => {
    render(<Page initial={{ category: "Development" }} installed={[demoCrm]} />);

    expect(screen.getByText("0 results in Development")).toBeTruthy();
    expect(screen.queryByText("Demo CRM")).toBeNull();
  });

  it("still draws them in their own category", () => {
    render(<Page initial={{ category: "Other" }} installed={[demoCrm]} />);

    expect(screen.getByText("1 result in Other")).toBeTruthy();
    expect(screen.getByText("Demo CRM")).toBeTruthy();
  });
});

describe("the Installed view under a category", () => {
  // A category that hides every installed plugin is a filter to clear, not a
  // workspace with nothing installed.
  it("says the filter matched nothing rather than that nothing is installed", () => {
    const demoCrm = { id: "demo-crm", name: "Demo CRM", description: "Contacts and deals." } as InstalledSkillRecord;
    render(<Page initial={{ category: "Development" }} installed={[demoCrm]} />);
    fireEvent.click(screen.getByRole("button", { name: /Installed/ }));

    expect(screen.getByText("No installed plugins matched your search.")).toBeTruthy();
    expect(screen.queryByText(/No plugins installed yet/)).toBeNull();
  });
});
