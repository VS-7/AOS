import { describe, expect, it, vi } from "vitest";
import { resolveSkill, skillSearch } from "./skill-scope";

const views = [
  { id: "contacts-table" },
  { id: "crm-table", skill: "crm" },
  { id: "twin", skill: "crm" },
  { id: "twin", skill: "sales" },
  { id: "shared-id" },
  { id: "shared-id", skill: "crm" },
];

describe("resolveSkill", () => {
  // A view or collection a skill brought lives under a skill-qualified key,
  // and get with its id alone answers NOT_FOUND. Every screen opened one by
  // id alone, so each landed on "Page not found".
  it("takes the skill the address names", async () => {
    const list = vi.fn();
    expect(await resolveSkill("crm-table", "crm", views, list)).toBe("crm");
    expect(list).not.toHaveBeenCalled();
  });

  it("finds the skill of an address that names none, from what is already known", async () => {
    const list = vi.fn();
    expect(await resolveSkill("crm-table", undefined, views, list)).toBe("crm");
    expect(await resolveSkill("contacts-table", "", views, list)).toBeUndefined();
    expect(list).not.toHaveBeenCalled();
  });

  // Without a skill the daemon answers the workspace's own entry; with two
  // skills and no workspace entry there is nothing to choose between.
  it("prefers the workspace's own entry, and guesses no skill between two", async () => {
    const list = vi.fn();
    expect(await resolveSkill("shared-id", undefined, views, list)).toBeUndefined();
    expect(await resolveSkill("twin", undefined, views, list)).toBeUndefined();
    expect(list).not.toHaveBeenCalled();
  });

  // Something an agent created after the list was loaded is not in it yet.
  it("asks the daemon when what is known does not have the id", async () => {
    const list = vi.fn(async () => [{ id: "fresh", skill: "crm" }]);
    expect(await resolveSkill("fresh", undefined, views, list)).toBe("crm");
    expect(await resolveSkill("fresh", undefined, undefined, list)).toBe("crm");
    expect(list).toHaveBeenCalledTimes(2);
  });

  it("gives up quietly when the daemon cannot list either", async () => {
    const list = vi.fn(async () => {
      throw new Error("offline");
    });
    expect(await resolveSkill("gone", undefined, [], list)).toBeUndefined();
  });
});

describe("skillSearch", () => {
  it("puts a skill in the address only when there is one", () => {
    expect(skillSearch("crm")).toEqual({ skill: "crm" });
    expect(skillSearch(undefined)).toEqual({});
    expect(skillSearch("")).toEqual({});
  });
});
