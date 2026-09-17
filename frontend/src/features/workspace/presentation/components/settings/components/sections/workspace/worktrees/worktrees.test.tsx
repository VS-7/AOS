import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

/**
 * A limit outside 1-50, or an emptied box, is refused by the schema — which
 * means the autosave simply does not happen. With nothing on screen to say
 * so, the page looked like it had saved (#218); and an emptied box held NaN,
 * which React refused as an input value.
 */

const daemon = vi.hoisted(() => ({
  workspace: {
    id: "ws-1",
    worktrees: { deleteOldWorktrees: true, worktreeLimit: 5, onCreateScript: "" },
  },
  save: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// Radix's switch measures itself; jsdom has no ResizeObserver.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

vi.mock("../../../../helpers/save-workspace-settings", () => ({
  saveWorkspaceSettings: (id: string, settings: Record<string, unknown>) => daemon.save(id, settings),
}));

vi.mock("@/app/aos", () => ({
  aos: {
    useForm: ({ schema, values, onSubmit }: any) => {
      const form = useForm({ resolver: zodResolver(schema), defaultValues: values, mode: "onChange" });
      const api = Object.assign(form, {
        isLoading: false,
        submit: form.handleSubmit(async (data: any) => {
          const next = await onSubmit(data);
          form.reset(next ?? data);
        }),
      });
      // The real aos.useForm saves as the person types (see the section's
      // `disableLoadingState` comment); handleSubmit refuses a value the
      // schema does not accept, which is what "not saved" means here.
      useEffect(() => {
        const watcher = form.watch((_values, info) => {
          if (info?.type === "change") void api.submit();
        });
        return () => watcher.unsubscribe();
        // eslint-disable-next-line react-hooks/exhaustive-deps
      }, []);
      return api;
    },
    stores: {
      workspace: {
        useState: (select: (s: any) => unknown) => select({ current: daemon.workspace }),
        get state() {
          return { current: daemon.workspace };
        },
      },
    },
  },
}));

import { WorkspaceWorktreesSection } from "./index";

afterEach(() => {
  cleanup();
  daemon.save.mockReset().mockResolvedValue(true);
});

describe("the worktrees section", () => {
  const limitBox = () => screen.getByRole("spinbutton", { name: /worktree limit/i }) as HTMLInputElement;

  it("sends only the limit that changed", async () => {
    render(<WorkspaceWorktreesSection />);

    fireEvent.change(limitBox(), { target: { value: "12" } });

    await waitFor(() =>
      expect(daemon.save).toHaveBeenCalledWith("ws-1", { "worktrees.worktreeLimit": 12 }),
    );
  });

  it("says why a limit out of range is not saved", async () => {
    render(<WorkspaceWorktreesSection />);

    fireEvent.change(limitBox(), { target: { value: "99" } });

    expect(await screen.findByRole("alert")).toHaveProperty(
      "textContent",
      "Choose a limit from 1 to 50.",
    );
    expect(daemon.save).not.toHaveBeenCalled();
  });

  it("leaves an emptied box empty, and does not save it", async () => {
    render(<WorkspaceWorktreesSection />);

    fireEvent.change(limitBox(), { target: { value: "" } });

    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(limitBox().value).toBe("");
    expect(daemon.save).not.toHaveBeenCalled();
  });
});
