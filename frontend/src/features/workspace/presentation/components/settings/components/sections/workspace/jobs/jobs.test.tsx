import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";

const state = vi.hoisted(() => ({ stats: null as unknown, jobs: [] as unknown[] }));

vi.mock("@/app/aos", () => ({
  aos: {
    stores: { workspace: { useState: (select: (s: unknown) => unknown) => select({ current: { id: "vs" } }) } },
    client: {
      job: {
        stats: { useQuery: () => ({ data: state.stats, isFetching: false, refetch: vi.fn() }) },
        list: { useQuery: () => ({ data: { jobs: state.jobs }, isLoading: false, refetch: vi.fn() }) },
        recover: { useMutation: () => ({ mutate: vi.fn(), loading: false }) },
        purge: { useMutation: () => ({ mutate: vi.fn(), loading: false }) },
      },
    },
  },
}));
vi.mock("@/components/ui/alert-provider", () => ({ useAlert: () => ({ confirm: vi.fn() }) }));

import { WorkspaceJobsSection } from "./index";

beforeEach(() => {
  cleanup();
  state.stats = { total: 0, byStatus: {} };
  state.jobs = [];
});

// Nothing in this build puts a turn, a routine run or a task run in the
// queue, and the page said "Every turn, routine and background task runs
// through this queue" above a list that stayed empty however many turns ran —
// with a Recover button that could never be pressed and a Purge that always
// removed nothing.
describe("WorkspaceJobsSection", () => {
  it("does not claim that turns run through the queue", () => {
    render(<WorkspaceJobsSection />);
    expect(screen.queryByText(/every turn/i)).toBeNull();
    expect(screen.getByText(/start directly/i)).toBeTruthy();
    // The daemon's tick queues a scheduled routine's run and its worker runs
    // it; the worker used to be built and never started, and the tick used to
    // take every run itself.
    expect(screen.getByText(/scheduled routine runs wait here/i)).toBeTruthy();
  });

  it("offers no action there is nothing to act on", () => {
    render(<WorkspaceJobsSection />);
    expect(screen.queryByRole("button", { name: /recover stalled/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /purge finished/i })).toBeNull();
    expect(screen.getByRole("button", { name: /refresh/i })).toBeTruthy();
  });

  it("offers each action once there is something for it", () => {
    state.stats = { total: 3, byStatus: { succeeded: 2, claimed: 1 }, stale: ["j-1"] };
    render(<WorkspaceJobsSection />);
    expect(screen.getByRole("button", { name: /recover stalled/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /purge finished/i })).toBeTruthy();
  });
});
