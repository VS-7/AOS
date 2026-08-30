import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, cleanup } from "@testing-library/react";

const invoke = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());

vi.mock("@/lib/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/client")>("@/lib/client");
  return { ...actual, client: { invoke }, isDesktop: () => false };
});
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }));

import { ApprovalDialog } from "./approval-dialog";

const waiting = {
  id: "req-1",
  agent: "luara",
  tool: "Bash",
  input: { command: "git push --force" },
  reason: "the hook asks before anything that rewrites history",
  risk: "high" as const,
  createdAt: "2026-08-29T23:00:00Z",
  expiresAt: "2099-01-01T00:00:00Z",
};

beforeEach(() => {
  cleanup();
  invoke.mockReset();
  toastError.mockReset();
});

describe("ApprovalDialog", () => {
  it("shows nothing when no tool call is waiting", async () => {
    invoke.mockResolvedValue({ pending: [], total: 0 });
    render(<ApprovalDialog />);
    await waitFor(() => expect(invoke).toHaveBeenCalledWith("approvals_list", expect.anything()));
    expect(screen.queryByText(/asking permission/i)).toBeNull();
  });

  // The whole point of the channel: the person has to see what is being
  // decided. A dialog that named the tool but hid the arguments would be
  // consent to something nobody was shown.
  it("shows the tool, the payload and the hook's reason", async () => {
    invoke.mockResolvedValue({ pending: [waiting], total: 1 });
    render(<ApprovalDialog />);

    await screen.findByText(/asking permission/i);
    expect(screen.getByText(/luara wants to run Bash/i)).toBeTruthy();
    expect(screen.getByText(/rewrites history/i)).toBeTruthy();
    expect(screen.getByText(/git push --force/)).toBeTruthy();
    expect(screen.getByText(/high risk/i)).toBeTruthy();
  });

  // ADR-0007: session is what a plain approval means. "A user approving an
  // action once does NOT authorize it in all contexts."
  it("approves for the session, not forever", async () => {
    invoke.mockResolvedValue({ pending: [waiting], total: 1 });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);

    invoke.mockResolvedValue({ id: "req-1", settled: true });
    fireEvent.click(screen.getByRole("button", { name: /approve once/i }));

    await waitFor(() =>
      expect(invoke).toHaveBeenCalledWith(
        "approvals_decide",
        expect.objectContaining({ id: "req-1", approved: true, remember: "session" }),
      ),
    );
  });

  // "always" is the one that writes a standing permission, so it takes a
  // second, deliberate click rather than sitting one slip away from the first.
  it("makes always-allow take a second click", async () => {
    invoke.mockResolvedValue({ pending: [waiting], total: 1 });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);

    invoke.mockClear();
    fireEvent.click(screen.getByRole("button", { name: /always allow/i }));
    expect(invoke).not.toHaveBeenCalled();

    invoke.mockResolvedValue({ id: "req-1", settled: true });
    fireEvent.click(screen.getByRole("button", { name: /click again to always allow/i }));
    await waitFor(() =>
      expect(invoke).toHaveBeenCalledWith(
        "approvals_decide",
        expect.objectContaining({ approved: true, remember: "always" }),
      ),
    );
  });

  it("denies without remembering anything", async () => {
    invoke.mockResolvedValue({ pending: [waiting], total: 1 });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);

    invoke.mockResolvedValue({ id: "req-1", settled: true });
    fireEvent.click(screen.getByRole("button", { name: /^deny$/i }));

    await waitFor(() =>
      expect(invoke).toHaveBeenCalledWith(
        "approvals_decide",
        expect.objectContaining({ approved: false, remember: "none" }),
      ),
    );
  });

  // `settled: false` means nothing was waiting under that id — the deadline
  // passed while the dialog was open. Reporting that as an approval would be
  // a lie: the call was refused.
  it("says so when the request had already expired", async () => {
    invoke.mockResolvedValue({ pending: [waiting], total: 1 });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);

    invoke.mockResolvedValue({ id: "req-1", settled: false });
    fireEvent.click(screen.getByRole("button", { name: /approve once/i }));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(String(toastError.mock.calls[0][0])).toMatch(/expired/i);
  });

  it("says how many more are waiting behind this one", async () => {
    invoke.mockResolvedValue({
      pending: [waiting, { ...waiting, id: "req-2" }, { ...waiting, id: "req-3" }],
      total: 3,
    });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);
    expect(screen.getByText(/2 more waiting/i)).toBeTruthy();
  });

  // A request past its deadline is already a denial. Offering buttons for it
  // would send a decision the broker refuses and read as if it had landed.
  it("offers no decision for a request that has run out", async () => {
    invoke.mockResolvedValue({
      pending: [{ ...waiting, expiresAt: "2020-01-01T00:00:00Z" }],
      total: 1,
    });
    render(<ApprovalDialog />);
    await screen.findByText(/asking permission/i);

    expect(screen.getByText(/has expired/i)).toBeTruthy();
    expect(screen.getByRole("button", { name: /approve once/i }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("button", { name: /^deny$/i }).hasAttribute("disabled")).toBe(true);
  });
});
