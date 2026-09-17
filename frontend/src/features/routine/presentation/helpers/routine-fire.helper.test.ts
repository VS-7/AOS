import { beforeEach, describe, expect, it, vi } from "vitest";

const invoke = vi.fn();

vi.mock("@/lib/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/client")>()),
  client: { invoke: (...a: unknown[]) => invoke(...a) },
}));

const { describeFireFailure } = await import("./routine-fire.helper");

beforeEach(() => invoke.mockReset());

describe("describeFireFailure", () => {
  it("shows the reason the recorded run failed with, which the refusal itself does not carry", async () => {
    invoke.mockResolvedValueOnce({
      routine: "r-1",
      runs: [{ id: "run-2", status: "failed", error: "AOS_AGENT_PROVIDER_FAILED: the codex provider did not answer" }],
      total: 2,
    });

    const failure = await describeFireFailure(
      "r-1",
      Object.assign(new Error("the routine ran and failed"), { code: "AOS_ROUTINE_RUN_FAILED" }),
    );

    expect(invoke).toHaveBeenCalledWith("routines_runs", expect.objectContaining({ id: "r-1", limit: 1 }));
    expect(failure.description).toBe("AOS_AGENT_PROVIDER_FAILED: the codex provider did not answer");
  });

  it("falls back to the refusal's own sentence when the run history cannot be read", async () => {
    invoke.mockRejectedValueOnce(Object.assign(new Error("gone"), { code: "AOS_ROUTINE_NOT_FOUND" }));

    const failure = await describeFireFailure(
      "r-1",
      Object.assign(new Error("the routine ran and failed"), { code: "AOS_ROUTINE_RUN_FAILED" }),
    );

    expect(failure.description).toBe("the routine ran and failed");
  });

  it("uses the refusal's message as it is when the routine never ran", async () => {
    const failure = await describeFireFailure(
      "r-1",
      Object.assign(new Error("this routine is disabled, so it did not run"), { code: "AOS_ROUTINE_DISABLED" }),
    );

    expect(invoke).not.toHaveBeenCalled();
    expect(failure.description).toBe("this routine is disabled, so it did not run");
  });
});
