import { describe, expect, it } from "vitest";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import { buildFormValues, buildUpdateBody, routineFormSchema } from "./routine-form.helper";

const routine: Routine = {
  id: "r-1",
  name: "Deploy hook",
  agent: "api-builder",
  content: "<rules>\n- item\n<!-- note -->\n{{ var }} \\\n</rules>",
  status: "enabled",
  triggers: [
    { type: "webhook", config: { tokenHash: "8467ed45" } },
    { type: "scheduled", config: { cron: "15 9 * * 1" } },
  ],
  scope: { allowCreateTasks: false, allowExternalCalls: false, allowedTools: ["tasks_list"] },
  createdAt: "2026-09-13T09:00:00Z",
  updatedAt: "2026-09-13T09:00:00Z",
};

describe("buildUpdateBody", () => {
  // Every save resent the triggers, and Go minted a new webhook token for
  // every webhook it was handed: a rename rotated the secret.
  it("sends a rename and nothing else", () => {
    const values = { ...buildFormValues(routine), name: "Deploy hook v2" };
    expect(buildUpdateBody(values, routine)).toEqual({ name: "Deploy hook v2" });
  });

  // What the form actually submits is the parsed value, and zod rebuilds
  // every object in its own field order. Comparing the two as JSON text made
  // a no-op Save resend the schedule — and, on a routine that also filters an
  // activity, its filters.
  it("sends a rename and nothing else, from the value the form submits", () => {
    const parsed = routineFormSchema.parse({ ...buildFormValues(routine), name: "Deploy hook v2" });
    expect(buildUpdateBody(parsed, routine)).toEqual({ name: "Deploy hook v2" });
  });

  it("sends nothing at all for a Save that changed nothing", () => {
    const withActivity: Routine = {
      ...routine,
      triggers: [
        ...routine.triggers,
        {
          type: "activity",
          config: { namespace: "tasks", event: "created" },
          filters: [{ field: "status", operator: "eq", value: "todo" }],
        },
      ],
    } as Routine;
    const parsed = routineFormSchema.parse(buildFormValues(withActivity));
    expect(buildUpdateBody(parsed, withActivity)).toEqual({});
  });

  // The collections codec ends a stored file with a newline, so a prompt
  // typed without one is read back with one. The form keeps what was typed,
  // and that difference used to resend the prompt on every later save.
  it("does not resend a prompt that only gained the stored trailing newline", () => {
    const typed = routine.content ?? "";
    const stored: Routine = { ...routine, content: `${typed}\n` };
    const values = { ...buildFormValues(stored), prompt: typed };
    expect(buildUpdateBody(values, stored)).toEqual({});
  });

  it("still sends a prompt that gained a blank line", () => {
    const typed = routine.content ?? "";
    const stored: Routine = { ...routine, content: `${typed}\n` };
    const values = { ...buildFormValues(stored), prompt: `${typed}\n\n` };
    expect(buildUpdateBody(values, stored)).toEqual({ prompt: `${typed}\n\n` });
  });

  it("sends the triggers once they change", () => {
    const values = buildFormValues(routine);
    values.triggers = values.triggers.filter((trigger) => trigger.type !== "scheduled");
    expect(buildUpdateBody(values, routine)).toEqual({ triggers: [{ type: "webhook" }] });
  });

  // The prompt is what a model reads: a load and a save must not rewrite it.
  it("keeps a prompt byte for byte", () => {
    const values = buildFormValues(routine);
    expect(values.prompt).toBe(routine.content);
    expect(buildUpdateBody(values, routine)).toEqual({});
    expect(routineFormSchema.parse(values).prompt).toBe(routine.content);
  });

  it("keeps an allowlist set elsewhere when a permission changes", () => {
    const values = buildFormValues(routine);
    values.scope = { ...values.scope, allowCreateTasks: true };
    expect(buildUpdateBody(values, routine)).toEqual({
      scope: { allowCreateTasks: true, allowExternalCalls: false, allowedTools: ["tasks_list"] },
    });
  });
});

describe("routineFormSchema", () => {
  // A name of only spaces passed here and was refused by the daemon.
  it("refuses a blank name and prompt, and a routine with no agent", () => {
    const result = routineFormSchema.safeParse({
      ...buildFormValues(null),
      name: "   ",
      prompt: " \n ",
      agent: "",
    });
    expect(result.success).toBe(false);
    const paths = result.success ? [] : result.error.issues.map((issue) => issue.path.join("."));
    expect(paths).toEqual(expect.arrayContaining(["name", "prompt", "agent"]));
  });
});
