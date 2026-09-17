import { describe, expect, it } from "vitest";
import { COMMAND_MAP } from "@/lib/command-map";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineTriggerFormSchema,
  RoutineTriggersHelper,
} from "./routine-triggers.helper";

/** What `routine.create`/`routine.update` put on the wire for `triggers`. */
const onTheWire = (triggers: unknown) => {
  const entry = COMMAND_MAP["routine.update"] as {
    coerceIn: Record<string, (v: unknown) => unknown>;
  };
  return (entry.coerceIn.triggers(triggers) as { triggers: unknown }).triggers;
};

/** A routine exactly as `routines_get` answers it. */
const stored = (triggers: Routine["triggers"]): Routine => ({
  id: "r-1",
  name: "Reviewed bugs",
  agent: "luara",
  content: "Check the evidence.",
  status: "enabled",
  triggers,
  createdAt: "2026-09-13T09:00:00Z",
  updatedAt: "2026-09-13T09:00:00Z",
});

describe("activity trigger filters", () => {
  // Go keeps an activity trigger's filters beside `config`, named `field`.
  // The editor read `config.filters` — always undefined — so a routine saved
  // by an agent or the CLI showed no filters, and saving it deleted them.
  it("load from where Go keeps them", () => {
    const [trigger] = RoutineTriggersHelper.buildFormTriggers(
      stored([
        {
          type: "activity",
          config: { namespace: "task", event: "status_changed" },
          filters: [
            { field: "to", operator: "eq", value: "in_review" },
            { field: "count", operator: "neq", value: 3 },
          ],
        },
      ]),
    );

    expect(trigger).toEqual({
      type: "activity",
      config: {
        namespace: "task",
        event: "status_changed",
        filters: [
          { field: "to", operator: "eq", value: "in_review" },
          { field: "count", operator: "neq", value: "3" },
        ],
      },
    });
  });

  // The editor sent `{path}`, and Go refused every save with
  // AOS_ROUTINE_FILTER_FIELD_REQUIRED: "a filter with no field compares nothing".
  it("reach Go named field", () => {
    const wire = onTheWire(
      RoutineTriggersHelper.toApiTriggers([
        {
          type: "activity",
          config: {
            namespace: "task",
            event: "status_changed",
            filters: [{ field: "to", operator: "eq", value: "finished" }],
          },
        },
      ]),
    );

    expect(wire).toEqual([
      {
        type: "activity",
        namespace: "task",
        event: "status_changed",
        filters: [{ field: "to", operator: "eq", value: "finished" }],
      },
    ]);
  });

  it("survive a load and a save unchanged", () => {
    const routine = stored([
      { type: "scheduled", config: { cron: "15 9 * * 1" } },
      { type: "webhook", config: { tokenHash: "8467ed45" } },
      {
        type: "activity",
        config: { namespace: "task", event: "status_changed" },
        filters: [{ field: "to", operator: "eq", value: "in_review" }],
      },
    ]);

    const wire = onTheWire(
      RoutineTriggersHelper.toApiTriggers(RoutineTriggersHelper.buildFormTriggers(routine)),
    );

    expect(wire).toEqual([
      { type: "scheduled", cron: "15 9 * * 1" },
      { type: "webhook" },
      {
        type: "activity",
        namespace: "task",
        event: "status_changed",
        filters: [{ field: "to", operator: "eq", value: "in_review" }],
      },
    ]);
  });

  it("refuse a filter with no field before the daemon has to", () => {
    const parsed = RoutineTriggerFormSchema.safeParse({
      type: "activity",
      config: { namespace: "task", event: "created", filters: [{ field: "", operator: "eq", value: "x" }] },
    });
    expect(parsed.success).toBe(false);
  });
});

describe("scheduled triggers", () => {
  // A cron set by an agent at :15 showed "Every week on Monday at [empty]":
  // the time picker offered only :00 and :30.
  it("keep a time that is not on the half hour", () => {
    expect(RoutineTriggersHelper.inferScheduledConfig("15 9 * * 1")).toEqual({
      preset: "weekly",
      time: "09:15",
      day: "1",
    });
    expect(RoutineTriggersHelper.timeOptions("09:15")).toContain("09:15");
    expect(RoutineTriggersHelper.timeOptions("09:00")).toHaveLength(48);
  });

  it("refuse a cron that is not five fields", () => {
    const parse = (cron: string) =>
      RoutineTriggerFormSchema.safeParse({
        type: "scheduled",
        config: { preset: "custom", cron, time: "09:00", day: "1" },
      }).success;

    expect(parse("*/5 * * * *")).toBe(true);
    expect(parse("not a cron")).toBe(false);
    expect(parse("")).toBe(false);
  });
});
