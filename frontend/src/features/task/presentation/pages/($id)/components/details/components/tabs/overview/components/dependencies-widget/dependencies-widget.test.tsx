import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Task, TaskWithContext } from "@/features/task/interfaces/task.interfaces";

/**
 * The widget names a dependency from the task list it already reads for its
 * picker — the newest 200 tasks. In a workspace with more than that, an older
 * dependency is missing from the page and used to be announced as deleted.
 */

const daemon = vi.hoisted(() => ({
  page: [] as Task[],
  getById: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));

vi.mock("@/app/aos", () => ({
  aos: {
    client: {
      task: {
        list: { useQuery: () => ({ data: { tasks: daemon.page, total: 200 } }) },
        getById: { query: daemon.getById },
      },
    },
  },
}));

import { DependenciesWidget } from "./index";

const task = (id: string, name: string): Task =>
  ({ id, name, status: "todo", priority: "no_priority" }) as unknown as Task;

function renderWidget(dependsOn: string[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <DependenciesWidget
        task={{ id: "t-self", name: "self", status: "todo", dependsOn, blocked: [] } as unknown as TaskWithContext}
        actions={{ setDependencies: vi.fn() } as never}
      />
    </QueryClientProvider>,
  );
}

describe("DependenciesWidget", () => {
  afterEach(() => {
    cleanup();
    daemon.getById.mockReset();
    daemon.page = [];
  });

  it("names a dependency the newest-200 page does not carry", async () => {
    daemon.page = [task("t-new", "A recent task")];
    daemon.getById.mockResolvedValue({ data: { task: task("t-old", "An older dependency") } });

    renderWidget(["t-old"]);

    expect(await screen.findByText("An older dependency")).toBeTruthy();
    expect(screen.queryByText("A task that no longer exists")).toBeNull();
    expect(daemon.getById).toHaveBeenCalledWith({ params: { task: "t-old" } });
  });

  it("still says so when the daemon has no such task", async () => {
    daemon.page = [task("t-new", "A recent task")];
    daemon.getById.mockResolvedValue({ data: undefined, error: { code: "AOS_TASK_NOT_FOUND" } });

    renderWidget(["t-gone"]);

    expect(await screen.findByText("A task that no longer exists")).toBeTruthy();
  });

  it("does not read back a dependency the page already names", async () => {
    daemon.page = [task("t-new", "A recent task")];

    renderWidget(["t-new"]);

    expect(await screen.findByText("A recent task")).toBeTruthy();
    await waitFor(() => expect(daemon.getById).not.toHaveBeenCalled());
  });
});
