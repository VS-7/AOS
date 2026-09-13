import { describe, expect, it } from "vitest";
import { COMMAND_MAP } from "./command-map";

type Coercions = Record<string, (value: unknown) => unknown>;

const coercionsOf = (path: string): Coercions =>
  ((COMMAND_MAP[path] as { coerceIn?: Coercions }).coerceIn ?? {}) as Coercions;

// goals_list's Query takes status as []Status and limit as int. The Goals
// page sent the URL's comma-joined status string and the project's Goals tab
// sent limit "50": both were refused as undecodable, so a status filter
// emptied the list and the Goals tab never showed the project's goals.
describe("goal.list sends what goals_list decodes", () => {
  const coerce = coercionsOf("goal.list");

  it("splits a comma-joined status into the list Go takes", () => {
    expect(coerce.status?.("active,paused")).toEqual(["active", "paused"]);
    expect(coerce.status?.(["achieved"])).toEqual(["achieved"]);
  });

  it("drops an empty status rather than filtering by nothing", () => {
    expect(coerce.status?.("")).toBeUndefined();
    expect(coerce.status?.(" , ")).toBeUndefined();
  });

  it("sends limit as a number", () => {
    expect(coerce.limit?.("50")).toBe(50);
    expect(coerce.limit?.(50)).toBe(50);
  });

  it("sends a single project, as Go filters by one", () => {
    expect(coerce.project?.(["api-de-teste"])).toBe("api-de-teste");
    expect(coerce.project?.("api-de-teste")).toBe("api-de-teste");
  });
});
