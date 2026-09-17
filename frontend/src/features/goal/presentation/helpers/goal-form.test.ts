import { describe, expect, it } from "vitest";
import { goalCreateBody, goalUpdateBody, prefillProject, type GoalFormFields } from "./goal-form";

const filled: GoalFormFields = {
  title: " Launch V1 ",
  description: "Ship it",
  content: "# Notes",
  measure: "released",
  priority: "high",
  project: "api-de-teste",
  deadline: "2026-08-31T18:00:00-03:00",
  status: "active",
};

describe("goalUpdateBody", () => {
  // Emptying Description and choosing "No project" toasted "Goal updated."
  // and changed nothing: the empty values were sent as undefined, JSON
  // dropped them, and Go read the missing keys as "leave unchanged".
  it("sends a cleared field as an empty string, which Go reads as clear", () => {
    const body = goalUpdateBody(
      { ...filled, description: "", content: "", measure: " ", project: "", deadline: "" },
      filled,
    );
    expect(body).toMatchObject({ description: "", content: "", measure: "", project: "", deadline: "" });
    expect(Object.keys(body)).toEqual(expect.arrayContaining(["description", "content", "measure", "project", "deadline"]));
  });

  // Every save sent the deadline back truncated to a date, rewriting an
  // agent's 18:00-03:00 to midnight UTC — a day earlier to anyone west of it.
  it("leaves an unchanged deadline out, so saving does not rewrite it", () => {
    const body = goalUpdateBody({ ...filled, title: "Renamed" }, filled);
    expect(body).not.toHaveProperty("deadline");
    expect(body.title).toBe("Renamed");
  });

  it("sends a deadline that changed", () => {
    const body = goalUpdateBody({ ...filled, deadline: "2026-09-20T00:00:00.000Z" }, filled);
    expect(body.deadline).toBe("2026-09-20T00:00:00.000Z");
  });
});

describe("goalCreateBody", () => {
  it("leaves out what was never filled in", () => {
    const body = goalCreateBody({ ...filled, description: "", content: "", measure: "", project: "", deadline: "" });
    expect(body).toEqual({ title: "Launch V1", priority: "high", status: "active" });
  });

  it("carries everything that was", () => {
    expect(goalCreateBody(filled)).toEqual({
      title: "Launch V1",
      description: "Ship it",
      content: "# Notes",
      measure: "released",
      priority: "high",
      project: "api-de-teste",
      deadline: "2026-08-31T18:00:00-03:00",
      status: "active",
    });
  });
});

describe("prefillProject", () => {
  const projects = [{ id: "p-real" }, { id: "p-other" }];

  it("preselects a project the workspace has", () => {
    expect(prefillProject(projects, "p-real")).toEqual({ project: "p-real" });
  });

  // /goals/new?project=nao-existe showed "No project" in the widget and still
  // sent project:"nao-existe" on create, which goals_create accepts.
  it("ignores an id the workspace does not have", () => {
    expect(prefillProject(projects, "nao-existe")).toEqual({});
  });

  it("has nothing to preselect without the link", () => {
    expect(prefillProject(projects, undefined)).toEqual({});
    expect(prefillProject(projects, "")).toEqual({});
  });
});
