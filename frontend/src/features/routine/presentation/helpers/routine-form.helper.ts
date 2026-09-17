import { z } from "zod";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineTriggerFormSchema,
  RoutineTriggersHelper,
} from "@/features/routine/presentation/helpers/routine-triggers.helper";

// Messages are catalogue keys: FormMessage translates what it renders.
// `refine` rather than `.trim()`: trim would rewrite the value itself, and a
// prompt is saved byte for byte.
export const routineFormSchema = z.object({
  name: z.string().refine((value) => value.trim() !== "", "Name is required"),
  prompt: z.string().refine((value) => value.trim() !== "", "Prompt is required"),
  agent: z.string().min(1, "Agent is required"),
  status: z.enum(["enabled", "disabled"]),
  triggers: z.array(RoutineTriggerFormSchema),
  scope: z.object({
    allowCreateTasks: z.boolean(),
    allowExternalCalls: z.boolean(),
  }),
});

export type RoutineFormValues = z.infer<typeof routineFormSchema>;

export function buildFormValues(routine: Routine | null): RoutineFormValues {
  return {
    name: routine?.name ?? "",
    prompt: routine?.content ?? "",
    agent: routine?.agent ?? "",
    status: routine?.status ?? "enabled",
    triggers: RoutineTriggersHelper.buildFormTriggers(routine),
    scope: {
      allowCreateTasks: routine?.scope?.allowCreateTasks ?? false,
      allowExternalCalls: routine?.scope?.allowExternalCalls ?? false,
    },
  };
}

/**
 * The same value, whatever order the keys were written in.
 *
 * The two sides of every comparison below are built by different hands: the
 * form's values come out of the zod schema, which orders a scheduled config
 * `{preset, cron, time, day}`, while `buildFormValues` writes it
 * `{cron, preset, time, day}`. `JSON.stringify` is key-order sensitive, so
 * those two never matched and a no-op Save resent the triggers of every
 * routine that has a schedule — with each activity filter's value
 * stringified on the way through.
 */
function stableOrder(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stableOrder);
  if (value && typeof value === "object") {
    const record = value as Record<string, unknown>;
    return Object.fromEntries(
      Object.keys(record)
        .sort()
        .map((key) => [key, stableOrder(record[key])]),
    );
  }
  return value;
}

const same = (a: unknown, b: unknown) =>
  JSON.stringify(stableOrder(a)) === JSON.stringify(stableOrder(b));

/**
 * A prompt without the trailing newline the store adds to it.
 *
 * `collections`' codec writes a final newline, so a prompt typed without one
 * is read back with one while the form still holds what was typed. Compared
 * literally, that difference resent the prompt on every later save. Only one
 * newline is discounted: a prompt that really ends in a blank line still
 * differs from one that does not.
 */
const withoutStoredNewline = (prompt: string) => prompt.replace(/\n$/, "");

/**
 * What an edit sends: only what changed.
 *
 * The page sent every field on every save. Triggers are replaced whole by Go,
 * so resending them is what used to rotate a webhook's token on a rename, and
 * the owner went along as a field Go reads as the lookup key.
 */
export function buildUpdateBody(
  values: RoutineFormValues,
  routine: Routine,
): Record<string, unknown> {
  const initial = buildFormValues(routine);
  const body: Record<string, unknown> = {};
  if (values.name !== initial.name) body.name = values.name;
  if (withoutStoredNewline(values.prompt) !== withoutStoredNewline(initial.prompt)) {
    body.prompt = values.prompt;
  }
  if (!same(values.triggers, initial.triggers)) {
    body.triggers = RoutineTriggersHelper.toApiTriggers(values.triggers);
  }
  if (!same(values.scope, initial.scope)) {
    body.scope = { ...routine.scope, ...values.scope };
  }
  return body;
}

