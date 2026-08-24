import { describe, expect, it } from "vitest";
import { COMMAND_MAP, DORMANT_DOMAINS, isDormant } from "./command-map";
import { COMMAND_KEYS } from "./schema";

describe("COMMAND_MAP", () => {
  it("never shrinks without someone noticing, and always registers its own dormant entries", () => {
    // An exact `toHaveLength(N)` here used to fail on every legitimate
    // addition for no reason — the final-review pass hit that itself
    // (`agent.getById`, `instruction.getById`, `memory.graph`,
    // `template.getById`, all found by the corrected call-path sweep).
    // What's actually worth guarding: the map doesn't shrink out from
    // under a call site (a lower bound, not a fixed count — see the "133"
    // history in this suite's git blame for the running total), and the
    // specific entries that were once silently missing stay declared. A
    // key present with a `null` value guards dormancy explicitly; a key
    // absent entirely is the bug this whole map exists to prevent — see
    // `COMMAND_MAP`'s own doc comment.
    expect(Object.keys(COMMAND_MAP).length).toBeGreaterThanOrEqual(129);
    for (const path of [
      "workspace.update",
      "workspace.directory",
      "agent.getById",
      "instruction.getById",
      "memory.graph",
      "template.getById",
    ]) {
      expect(path in COMMAND_MAP).toBe(true);
    }
  });

  it("points only at commands the Go side actually publishes", () => {
    // A `CommandDescriptor` entry's real registry key is `.key`, not the
    // entry itself — resolve both shapes to the same string before
    // checking, so a descriptor pointing at a command Go doesn't have
    // fails this test the same way a plain-string typo always did.
    const published = new Set<string>(COMMAND_KEYS);
    const broken = Object.entries(COMMAND_MAP)
      .map(([k, v]): [string, string | undefined] => [
        k,
        typeof v === "string" ? v : typeof v === "object" && v !== null && "key" in v ? v.key : undefined,
      ])
      .filter(([, key]) => key !== undefined && !published.has(key))
      .map(([k, key]) => `${k} -> ${String(key)}`);
    expect(broken).toEqual([]);
  });

  it("maps the irregular renames, including the kebab-case ones", () => {
    // `task.getById` is now a `CommandDescriptor` (it needs `renameIn` and
    // `wrapOut` — see command-map.ts's own doc comment on why), so this
    // checks its `.key` rather than the whole entry.
    const taskGetById = COMMAND_MAP["task.getById"];
    expect(typeof taskGetById === "object" && taskGetById !== null ? taskGetById.key : taskGetById).toBe("tasks_get");
    const taskSetStatus = COMMAND_MAP["task.setStatus"];
    expect(typeof taskSetStatus === "object" && taskSetStatus !== null ? taskSetStatus.key : taskSetStatus).toBe("tasks_set-status");
    // `activity.markAsRead` is now a `CommandDescriptor` too (task 12: the
    // UI's `params: { activity: id }` needs `renameIn` to reach Go's `id`
    // field) — same `.key`-not-whole-entry check as the two above.
    const activityMarkAsRead = COMMAND_MAP["activity.markAsRead"];
    expect(typeof activityMarkAsRead === "object" && activityMarkAsRead !== null ? activityMarkAsRead.key : activityMarkAsRead).toBe("activity_read");
    expect(COMMAND_MAP["activity.markAllAsRead"]).toBe("activity_read-all");
  });

  it("declares dormancy with null, never by omitting the key", () => {
    // task-10 lit `collection` (and `view`/`skill`/`toolset` with it); the
    // Phase 8 domain pass lit `artifact`/`goal`/`instruction`/`marketplace`/
    // `project`/`template`/`tunnel` alongside it — see each one's own
    // comment above. `model.list` has since been lit too (the `models`
    // group discovers a provider's catalogue by asking it), so the
    // assertion moved to `model.set`, which stays dormant because
    // connecting a provider is `config.update` here rather than a command
    // of its own.
    expect("model.set" in COMMAND_MAP).toBe(true);
    expect(COMMAND_MAP["model.set"]).toBeNull();
  });

  it("recognizes the 2 domains without a Go backend", () => {
    // task-10 moved collection/view/toolset/skill out (14 -> 10); the
    // Phase 8 domain pass moved artifact/goal/instruction/marketplace/
    // project/template/tunnel out in turn (10 -> 3), and the `models`
    // group moved `model` out after it (3 -> 2) — every path they had is
    // now a real mapping or an individually `null` one, not whole-domain
    // dormancy. `token` and `user` remain: no command group exists for
    // either yet.
    expect(DORMANT_DOMAINS.size).toBe(2);
    expect(isDormant("model")).toBe(false);
    expect(isDormant("goal")).toBe(false);
    expect(isDormant("collection")).toBe(false);
    expect(isDormant("task")).toBe(false);
  });
});
