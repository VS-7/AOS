import { describe, expect, it, vi } from "vitest";
import { loadRoutinePage } from "./loader";

const notFound = Symbol("not found");

function daemon(routine: unknown) {
  return {
    activity: { listEvents: { query: vi.fn().mockResolvedValue({ data: [] }) } },
    routine: {
      getById: vi.fn().mockResolvedValue(routine ? { data: { routine } } : { data: undefined, error: { code: "AOS_ROUTINE_NOT_FOUND" } }),
      runs: vi.fn().mockResolvedValue({ data: { runs: [] } }),
    },
  };
}

function load(client: ReturnType<typeof daemon>, id = "r-1") {
  return loadRoutinePage({
    client: {
      activity: client.activity,
      routine: {
        getById: { query: client.routine.getById },
        runs: { query: client.routine.runs },
      },
    },
    request: { params: { id } },
    // The real one throws (app/builders/response.ts); here it is enough to
    // see that the loader stopped at it.
    response: { notFound: () => notFound as never },
  });
}

describe("the routine page's loader", () => {
  // Both commands used to be sent at once, so a routine deleted elsewhere
  // refused both and logged two NOT_FOUND errors for one missing routine.
  it("does not ask for the run history of a routine that is gone", async () => {
    const client = daemon(null);

    expect(await load(client)).toBe(notFound);
    expect(client.routine.getById).toHaveBeenCalledTimes(1);
    expect(client.routine.runs).not.toHaveBeenCalled();
  });

  it("reads the history of a routine that exists", async () => {
    const client = daemon({ id: "r-1", name: "Deploy hook" });

    const data = await load(client);

    expect(data).toMatchObject({ mode: "edit", routine: { id: "r-1" }, runs: [] });
    expect(client.routine.runs).toHaveBeenCalledWith({
      params: { routine: "r-1" },
      query: { limit: 200 },
    });
  });

  it("reads nothing about a routine that is being created", async () => {
    const client = daemon(null);

    expect(await load(client, "new")).toMatchObject({ mode: "create", routine: null });
    expect(client.routine.getById).not.toHaveBeenCalled();
    expect(client.routine.runs).not.toHaveBeenCalled();
  });
});
