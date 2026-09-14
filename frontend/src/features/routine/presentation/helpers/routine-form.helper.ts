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

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

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
  if (values.prompt !== initial.prompt) body.prompt = values.prompt;
  if (!same(values.triggers, initial.triggers)) {
    body.triggers = RoutineTriggersHelper.toApiTriggers(values.triggers);
  }
  if (!same(values.scope, initial.scope)) {
    body.scope = { ...routine.scope, ...values.scope };
  }
  return body;
}

