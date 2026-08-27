import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, cleanup } from "@testing-library/react";

const invoke = vi.hoisted(() => vi.fn());

vi.mock("@/lib/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/client")>("@/lib/client");
  return { ...actual, client: { invoke }, isDesktop: () => false, system: { pickFiles: vi.fn() } };
});

import { WorkspaceGate } from "./WorkspaceGate";
import { AppStateProvider } from "@/lib/app-state";

function App() {
  return <div>the application</div>;
}

/** The gate renders below AppStateProvider in App.tsx; the Logo it draws
 *  reads the appearance from there. */
function mount() {
  return render(
    <AppStateProvider>
      <WorkspaceGate>
        <App />
      </WorkspaceGate>
    </AppStateProvider>,
  );
}

beforeEach(() => {
  // This project does not enable vitest's `globals`, so testing-library's
  // automatic cleanup never registers — without this each test renders on top
  // of the last one's DOM and the queries below match twice.
  cleanup();
  invoke.mockReset();
});

describe("WorkspaceGate", () => {
  it("lets the application through when a workspace exists", async () => {
    invoke.mockResolvedValueOnce({ workspaces: [{ id: "meu-espaco" }] });

    mount();

    expect(await screen.findByText("the application")).toBeTruthy();
  });

  // The state the user was actually in: an account, no workspace, and no
  // button anywhere in the interface that creates one — the only one lives
  // inside the workspace switcher, which had nothing to switch between.
  it("asks for a first workspace when the installation has none", async () => {
    invoke.mockResolvedValueOnce({ workspaces: [] });

    mount();

    expect(await screen.findByRole("button", { name: /workspace/i })).toBeTruthy();
    expect(screen.queryByText("the application")).toBeNull();
  });

  it("creates the workspace and then opens the application", async () => {
    invoke
      .mockResolvedValueOnce({ workspaces: [] })
      .mockResolvedValueOnce({ workspace: { id: "acme" } })
      .mockResolvedValueOnce({ workspaces: [{ id: "acme" }] });

    mount();

    const name = await screen.findByLabelText(/workspace name/i);
    fireEvent.change(name, { target: { value: "Acme" } });
    fireEvent.click(screen.getByRole("button", { name: /create workspace/i }));

    expect(await screen.findByText("the application")).toBeTruthy();

    const create = invoke.mock.calls.find(([key]) => key === "workspace_create");
    expect(create?.[1]).toMatchObject({ name: "Acme" });
    // No `path` when none was given: the daemon then puts the workspace under
    // its own state directory, which is the right default for somebody who
    // opened an application rather than a repository.
    expect(create?.[1]).not.toHaveProperty("path");
  });

  it("refuses to create one with no name", async () => {
    invoke.mockResolvedValueOnce({ workspaces: [] });

    mount();

    fireEvent.click(await screen.findByRole("button", { name: /create workspace/i }));

    expect(await screen.findByText(/name your workspace/i)).toBeTruthy();
    expect(invoke.mock.calls.filter(([key]) => key === "workspace_create")).toHaveLength(0);
  });

  // A daemon that has not answered yet is not an installation without a
  // workspace. Guessing wrong here would put a creation form in front of
  // somebody whose workspaces are perfectly fine.
  it("lets the application through when the daemon cannot be reached", async () => {
    invoke.mockRejectedValueOnce(new Error("connection refused"));

    mount();

    await waitFor(() => expect(screen.getByText("the application")).toBeTruthy());
  });
});
