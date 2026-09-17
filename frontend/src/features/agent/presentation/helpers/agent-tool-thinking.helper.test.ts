import { readFileSync } from "node:fs";
import { afterEach, describe, it, expect } from "vitest";
import en from "@/lib/i18n/locales/en.json";
import ptBR from "@/lib/i18n/locales/pt-BR.json";
import { setLocale } from "@/lib/i18n";
import { AgentToolThinkingHelper, DOMAIN_COMMAND_TITLES } from "./agent-tool-thinking.helper";

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
    expect(AgentToolThinkingHelper.getToolConfig("tasks_branch").title).toBe("Tasks · Create branch");
    expect(AgentToolThinkingHelper.getToolConfig("Bash").title).toBe("Run command");
  });

  // The verb alone decided: "read" is a read, so marking the inbox as read —
  // a row titled "Activity · Mark as read" — was counted among the reads, and
  // registering a repository ("introspect") as one too.
  it("counts a command that changes records as a write, whatever its verb", () => {
    expect(AgentToolThinkingHelper.getToolConfig("activity_read").action).toBe("write");
    expect(AgentToolThinkingHelper.getToolConfig("activity_read-all").action).toBe("write");
    expect(AgentToolThinkingHelper.getToolConfig("workspace_introspect").action).toBe("write");
    expect(AgentToolThinkingHelper.getToolConfig("memories_reflect").action).toBe("read");
    expect(AgentToolThinkingHelper.getToolConfig("activity_get").action).toBe("read");
  });

  // Ten registered verbs ("decide", "call", "check", "runs", …) were in no
  // group, so their rows fell into "other actions" with a generic description.
  it("classifies every command the daemon registers", () => {
    const schema = readFileSync("src/lib/schema.ts", "utf8");
    const commands = [...schema.matchAll(/^  "([a-z][a-z0-9-]*_[a-z0-9_-]+)": \{/gm)].map((m) => m[1]);
    expect(commands.filter((name) => AgentToolThinkingHelper.getToolConfig(name).action === "other")).toEqual([]);
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

describe("a daemon command's title", () => {
  afterEach(() => setLocale("en"));

  // The verb went through the generic one-word keys, which already meant
  // something else: "List" is the noun "Lista", "Run" the noun "Execução",
  // "Rotate" is "Girar". Rows read "Tarefas · Lista".
  it("reads as the action in Portuguese, not as a noun that shares its English word", () => {
    setLocale("pt-BR");
    expect(AgentToolThinkingHelper.getToolConfig("tasks_list").title).toBe("Tarefas · Listar");
    expect(AgentToolThinkingHelper.getToolConfig("instructions_list").title).toBe("Instruções · Listar");
    expect(AgentToolThinkingHelper.getToolConfig("routines_rotate").title).toBe("Rotinas · Trocar token do webhook");
    expect(AgentToolThinkingHelper.getToolConfig("update_check").title).toBe("Atualização · Verificar");
    expect(AgentToolThinkingHelper.getToolConfig("approvals_decide").title).toBe("Aprovações · Decidir");
  });

  // About thirty Go verbs had no catalogue entry and rendered in English. The
  // registry is what an agent can call, so every command in it needs a title
  // of its own, translated.
  it("exists, translated, for every command the daemon registers", () => {
    const schema = readFileSync("src/lib/schema.ts", "utf8");
    const commands = [...schema.matchAll(/^  "([a-z][a-z0-9-]*_[a-z0-9_-]+)": \{/gm)].map((m) => m[1]);
    expect(commands.length).toBeGreaterThan(100);

    const untitled = commands.filter((name) => !(name in DOMAIN_COMMAND_TITLES));
    expect(untitled, "commands with no title").toEqual([]);

    const titles = Object.values(DOMAIN_COMMAND_TITLES);
    expect(titles.filter((key) => !(key in en)), "titles missing from en.json").toEqual([]);
    expect(titles.filter((key) => !(key in ptBR)), "titles missing from pt-BR.json").toEqual([]);
    // A title key names its group, so it can never be a generic word another
    // screen translates with a different meaning.
    expect(titles.filter((key) => !key.includes(" · "))).toEqual([]);
  });

  // The group half is the screen's own name for those records: toolset rows
  // read "Conjuntos de ferramentas" while the marketplace and settings call
  // them "Toolsets". "Update" is the verb elsewhere, so it has no noun to match.
  it("names its group the way the rest of the app does", () => {
    const catalogue = ptBR as Record<string, string>;
    const mismatched = Object.values(DOMAIN_COMMAND_TITLES).flatMap((key) => {
      const [group] = key.split(" · ");
      const noun = group === "Update" ? undefined : catalogue[group];
      const shown = catalogue[key]?.split(" · ")[0];
      return noun !== undefined && shown !== noun ? [`${key}: ${shown} ≠ ${noun}`] : [];
    });
    expect(mismatched).toEqual([]);
  });

  it("falls back to the command's own words for a tool the registry does not know", () => {
    setLocale("pt-BR");
    expect(AgentToolThinkingHelper.getToolConfig("github_create-issue").title).toBe("Github · Create issue");
  });
});

describe("every tool row", () => {
  it("offers details naming the tool that was called", () => {
    const [step] = AgentToolThinkingHelper.toThinkingSteps(message(tool("goals_get", { input: { goal: "api-de-biblioteca-funcional" } })));
    expect(step.details).toEqual(expect.arrayContaining(["tool: goals_get"]));
  });
});
