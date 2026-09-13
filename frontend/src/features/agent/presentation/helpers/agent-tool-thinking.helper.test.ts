import { describe, it, expect } from "vitest";
import { AgentToolThinkingHelper } from "./agent-tool-thinking.helper";

const tool = (name: string, extra: Record<string, unknown> = {}) => ({
  type: `tool-${name}`,
  toolName: name,
  state: "output-available",
  input: {},
  ...extra,
});
const message = (...parts: unknown[]) =>
  ({ id: "m-1", role: "assistant", parts, metadata: { type: "agent", data: { id: "api-builder" } } }) as never;

describe("a failed tool call", () => {
  // The row showed the agent's reasoning for making the call and offered no
  // Details, so "the isolated checkout could not be created" was never on
  // screen — only why the agent had tried.
  const failed = tool("tasks_branch", {
    state: "output-error",
    input: { task: "0186fedb", _reasoning: "The assigned task is configured for an isolated worktree." },
    output: { code: "AOS_TASK_WORKTREE_FAILED", message: "the isolated checkout could not be created" },
    errorText: "the isolated checkout could not be created",
  });

  it("says what went wrong where the description goes", () => {
    const [step] = AgentToolThinkingHelper.toThinkingSteps(message(failed));
    expect(step.description).toBe("the isolated checkout could not be created");
    expect(step.status).toBe("error");
  });

  it("keeps the error code and the reasoning in its details", () => {
    const [step] = AgentToolThinkingHelper.toThinkingSteps(message(failed));
    expect(step.details).toEqual(
      expect.arrayContaining([
        expect.stringContaining("AOS_TASK_WORKTREE_FAILED"),
        expect.stringContaining("isolated worktree"),
      ]),
    );
  });
});

describe("domain commands", () => {
  // Go's tools are its commands, named group_verb. Only legacy names were
  // mapped, so every one of them rendered as its raw name over "Used an agent
  // tool", and the header counted none of them.
  it("gets a readable title and the action its verb names", () => {
    expect(AgentToolThinkingHelper.getToolConfig("projects_get")).toMatchObject({ title: "Projects · Read", action: "read" });
    expect(AgentToolThinkingHelper.getToolConfig("tasks_list")).toMatchObject({ action: "read" });
    expect(AgentToolThinkingHelper.getToolConfig("todos_set-status")).toMatchObject({ title: "Todos · Set status", action: "write" });
    expect(AgentToolThinkingHelper.getToolConfig("memories_recall")).toMatchObject({ action: "search" });
    expect(AgentToolThinkingHelper.getToolConfig("routines_fire")).toMatchObject({ action: "execute" });
    expect(AgentToolThinkingHelper.getToolConfig("tasks_branch").title).toBe("Tasks · Branch");
    expect(AgentToolThinkingHelper.getToolConfig("Bash").title).toBe("Run command");
  });

  it("counts every call in the summary", () => {
    const summary = AgentToolThinkingHelper.getSummary(
      message(tool("projects_get"), tool("tasks_list"), tool("comments_create"), tool("tasks_branch", { state: "output-error" }), tool("mystery_thing")),
    );
    expect(summary.reads).toBe(2);
    expect(summary.writes).toBe(2);
    expect(summary.errors).toBe(1);
    expect(summary.reads + summary.writes + summary.searches + summary.executions + summary.browsing + summary.management + summary.other).toBe(summary.total);
    expect(summary.other).toBe(1);
  });
});

describe("every tool row", () => {
  it("offers details naming the tool that was called", () => {
    const [step] = AgentToolThinkingHelper.toThinkingSteps(message(tool("goals_get", { input: { goal: "api-de-biblioteca-funcional" } })));
    expect(step.details).toEqual(expect.arrayContaining(["tool: goals_get"]));
  });
});
