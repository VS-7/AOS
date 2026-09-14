import { afterEach, describe, expect, it } from "vitest";
import { setLocale } from "@/lib/i18n";
import {
  activityDayLabel,
  activityTarget,
  activityTimeLabel,
  activityTitle,
  isOwnActivity,
} from "./activity-presentation.helper";

afterEach(() => setLocale("en"));

const task = (event: string, data: Record<string, unknown>, title = "raw title") => ({
  id: "a1",
  namespace: "task",
  event,
  title,
  data,
});

describe("an activity's line", () => {
  // Go writes "Build the API moved to in_progress": English, with the raw
  // enum, whatever language the interface is in.
  it("names a task's new status the way the status tabs do", () => {
    const line = activityTitle(task("status_changed", { name: "Build the API", from: "stopped", to: "in_progress" }));
    expect(line).toBe("Build the API moved to In Progress");

    setLocale("pt-BR");
    expect(activityTitle(task("status_changed", { name: "Build the API", to: "in_progress" }))).toBe(
      "Build the API passou para Em andamento",
    );
    expect(activityTitle(task("created", { name: "Build the API" }))).toBe("Nova tarefa: Build the API");
  });

  it("covers goals, projects and agents from their own payloads", () => {
    expect(activityTitle({ id: "g", namespace: "goal", event: "created", title: "x", data: { title: "Ship" } })).toBe("New goal: Ship");
    expect(activityTitle({ id: "p", namespace: "project", event: "deleted", title: "x", data: { name: "Site" } })).toBe("Deleted project: Site");
    expect(activityTitle({ id: "a", namespace: "agent", event: "updated", title: "x", data: { name: "Luara" } })).toBe("Luara updated");
  });

  it("keeps the daemon's line for anything it does not know", () => {
    expect(activityTitle({ id: "r", namespace: "routine", event: "fired", title: "Nightly succeeded", data: {} })).toBe("Nightly succeeded");
    expect(activityTitle(task("status_changed", {}, "Fallback line"))).toBe("Fallback line");
  });
});

describe("where an activity opens", () => {
  // The toast's Open went to the task list for a task and to Home for
  // everything else, and routed chats/skills/templates to pages that do not
  // exist.
  it("opens the record it is about", () => {
    expect(activityTarget(task("status_changed", { task: "t-1" }))).toEqual({ to: "/tasks/$id", params: { id: "t-1" } });
    expect(activityTarget({ id: "g", namespace: "goal", event: "updated", title: "", data: { goal: "g-1" } })).toEqual({
      to: "/goals/$id",
      params: { id: "g-1" },
    });
    expect(activityTarget({ id: "p", namespace: "project", event: "created", title: "", data: { project: "p-1" } })).toEqual({
      to: "/projects/$id",
      params: { id: "p-1" },
    });
    expect(activityTarget({ id: "r", namespace: "routine", event: "fired", title: "", data: { routine: "nightly" } })).toEqual({
      to: "/routines/$id",
      params: { id: "nightly" },
    });
    expect(activityTarget({ id: "a", namespace: "agent", event: "created", title: "", data: { agent: "luara" } })).toEqual({
      to: "/settings/$group/$section",
      params: { group: "workspace", section: "agents" },
    });
  });

  it("opens the list when the record is gone, and the inbox when there is nothing to open", () => {
    expect(activityTarget(task("deleted", { task: "t-1" }))).toEqual({ to: "/tasks" });
    expect(activityTarget({ id: "x", namespace: "toolset", event: "call", title: "", data: {} })).toEqual({ to: "/activities" });
    expect(activityTarget(task("created", {}))).toEqual({ to: "/tasks" });
  });
});

describe("an activity's time", () => {
  const now = new Date(2026, 8, 13, 8, 0);

  // The group header compared calendar days and the row counted 24-hour
  // spans, so 23:00 yesterday seen at 08:00 sat under "Yesterday" labelled
  // "Today". Both read the calendar day now, and the row says the time.
  it("groups by calendar day", () => {
    expect(activityDayLabel(new Date(2026, 8, 13, 0, 5).toISOString(), now)).toBe("Today");
    expect(activityDayLabel(new Date(2026, 8, 12, 23, 0).toISOString(), now)).toBe("Yesterday");
    expect(activityDayLabel(new Date(2026, 7, 30, 12, 0).toISOString(), now)).toBe("Sunday, Aug 30");

    setLocale("pt-BR");
    expect(activityDayLabel(new Date(2026, 8, 12, 23, 0).toISOString(), now)).toBe("Ontem");
    expect(activityDayLabel(new Date(2026, 7, 30, 12, 0).toISOString(), now)).toMatch(/domingo/);
  });

  it("gives each row its time of day", () => {
    expect(activityTimeLabel(new Date(2026, 8, 12, 23, 0).toISOString())).toMatch(/11:00\s?PM/);
    setLocale("pt-BR");
    expect(activityTimeLabel(new Date(2026, 8, 12, 23, 0).toISOString())).toBe("23:00");
  });
});

describe("an activity you caused", () => {
  it("is yours only when the person signed in here did it", () => {
    expect(isOwnActivity({ actor: "u-1", actorType: "user" }, "u-1")).toBe(true);
    expect(isOwnActivity({ actor: "u-2", actorType: "user" }, "u-1")).toBe(false);
    expect(isOwnActivity({ actor: "u-1", actorType: "agent" }, "u-1")).toBe(false);
    expect(isOwnActivity({ actor: "u-1", actorType: "user" }, undefined)).toBe(false);
  });
});
