import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, renderHook, waitFor } from "@testing-library/react";

/**
 * People come from the account list, fetched the first time a picker needs
 * it and remembered for the window. "Remembered" must not include a failure:
 * the task dialog is mounted on every load, so one transient bridge error at
 * start-up used to leave every assignee picker listing only the signed-in
 * person until the window was reloaded.
 */

const daemon = vi.hoisted(() => ({ refreshDirectory: vi.fn() }));

vi.mock("@/app/aos", () => ({
  aos: {
    stores: {
      agent: { useState: (select: (s: unknown) => unknown) => select({ items: [] }) },
      auth: { useState: (select: (s: unknown) => unknown) => select({ user: null }) },
      workspace: {
        useState: (select: (s: unknown) => unknown) => select({ directory: { users: [] } }),
        actions: { refreshDirectory: () => daemon.refreshDirectory() },
      },
    },
  },
}));

describe("useAssigneeDirectory", () => {
  beforeEach(() => {
    vi.resetModules();
    daemon.refreshDirectory.mockReset();
  });
  afterEach(cleanup);

  it("asks again after a failed attempt", async () => {
    daemon.refreshDirectory.mockRejectedValueOnce(new Error("the bridge was not ready"));
    const { useAssigneeDirectory } = await import("./assignee-directory.hook");

    renderHook(() => useAssigneeDirectory()).unmount();
    await waitFor(() => expect(daemon.refreshDirectory).toHaveBeenCalledTimes(1));

    daemon.refreshDirectory.mockResolvedValueOnce(undefined);
    renderHook(() => useAssigneeDirectory());
    await waitFor(() => expect(daemon.refreshDirectory).toHaveBeenCalledTimes(2));
  });

  it("asks once while it is working, however many pickers mount", async () => {
    daemon.refreshDirectory.mockResolvedValue(undefined);
    const { useAssigneeDirectory } = await import("./assignee-directory.hook");

    renderHook(() => useAssigneeDirectory());
    renderHook(() => useAssigneeDirectory());
    await waitFor(() => expect(daemon.refreshDirectory).toHaveBeenCalledTimes(1));
  });
});
