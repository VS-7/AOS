import type { GoalPriority, GoalStatus } from "@/features/goal/interfaces/goal.interfaces";

/** What the goal page's form holds. */
export interface GoalFormFields {
  title: string;
  description?: string;
  content?: string;
  measure?: string;
  priority: GoalPriority;
  project?: string;
  /** The deadline as the daemon wrote it, or as the picker chose it; "" for none. */
  deadline?: string;
  status: GoalStatus;
}

/**
 * The body of goals_update.
 *
 * A field the person emptied is sent as "", because goals_update reads an
 * omitted key as "leave unchanged" and "" as "clear" — sending undefined
 * made every clear a silent no-op behind a "Goal updated." toast.
 *
 * The deadline is sent only when it changed. The form used to cut it to a
 * date and send it back on every save, which rewrote an instant an agent had
 * set (18:00-03:00) to midnight UTC.
 */
export function goalUpdateBody(values: GoalFormFields, original: GoalFormFields) {
  const body: Record<string, string> = {
    title: values.title.trim(),
    description: (values.description ?? "").trim(),
    content: (values.content ?? "").trim(),
    measure: (values.measure ?? "").trim(),
    priority: values.priority,
    project: (values.project ?? "").trim(),
    status: values.status,
  };
  const deadline = values.deadline ?? "";
  if (deadline !== (original.deadline ?? "")) body.deadline = deadline;
  return body;
}

/** The body of goals_create: only what was filled in. */
export function goalCreateBody(values: GoalFormFields) {
  const optional = {
    description: values.description?.trim(),
    content: values.content?.trim(),
    measure: values.measure?.trim(),
    project: values.project?.trim(),
    deadline: values.deadline,
  };
  return {
    title: values.title.trim(),
    ...Object.fromEntries(Object.entries(optional).filter(([, value]) => value)),
    priority: values.priority,
    status: values.status,
  } as Record<string, string>;
}
