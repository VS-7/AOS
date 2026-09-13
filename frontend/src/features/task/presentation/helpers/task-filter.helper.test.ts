import { describe, expect, it } from "vitest";
import { filterTasks, filterableTypes } from "./task-filter.helper";
import type { Task } from "@/features/task/interfaces/task.interfaces";

const task = (id: string, extra: Partial<Task>) => ({ id, name: id, priority: "no_priority", type: "", ...extra }) as Task;

const tasks = [
  task("a", { priority: "urgent", type: "bug", project: "api", goal: "g1" }),
  task("b", { priority: "high", type: "feature", project: "web" }),
  task("c", { priority: "low", type: "docs" }),
];

const none = { priorities: [], types: [], projects: [], goals: [] };

describe("filterTasks", () => {
  it("keeps everything when nothing is selected", () => {
    expect(filterTasks(tasks, none).map((t) => t.id)).toEqual(["a", "b", "c"]);
  });

  it("matches any of the selected values, not only the first", () => {
    expect(filterTasks(tasks, { ...none, priorities: ["urgent", "low"] }).map((t) => t.id)).toEqual(["a", "c"]);
    expect(filterTasks(tasks, { ...none, types: ["bug", "feature"] }).map((t) => t.id)).toEqual(["a", "b"]);
    expect(filterTasks(tasks, { ...none, projects: ["web", "api"] }).map((t) => t.id)).toEqual(["a", "b"]);
  });

  it("combines the filters", () => {
    expect(filterTasks(tasks, { ...none, types: ["bug", "feature"], priorities: ["high"] }).map((t) => t.id)).toEqual(["b"]);
    expect(filterTasks(tasks, { ...none, goals: ["g1"] }).map((t) => t.id)).toEqual(["a"]);
  });
});

describe("filterableTypes", () => {
  it("offers the workspace's types even when no loaded task has them", () => {
    const types = filterableTypes([{ id: "feature" }, { id: "bug" }, { id: "config" }], [tasks[0]]);
    expect(types).toEqual(["feature", "bug", "config"]);
  });

  it("keeps a type a task still carries after the workspace dropped it", () => {
    expect(filterableTypes([{ id: "bug" }], tasks)).toEqual(["bug", "docs", "feature"]);
  });
});
