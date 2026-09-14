import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";

export interface TaskFilters {
  priorities: readonly TaskPriority[];
  types: readonly string[];
  projects: readonly string[];
  goals: readonly string[];
}

const matches = (selected: readonly string[], value: string | undefined) =>
  selected.length === 0 || selected.includes(value ?? "");

/**
 * The task list's filter bar, applied to the tasks the page loaded.
 *
 * It used to be sent to tasks_list, whose filters are one value each: a
 * priority went as an array and was refused (AOS_COMMAND_INVALID_INPUT), which
 * emptied the list with no message, and two types or projects filtered by the
 * first alone. The loader now asks only for the search, and every selection
 * here means "any of these".
 */
export function filterTasks(tasks: readonly Task[], filters: TaskFilters): Task[] {
  return tasks.filter(
    (task) =>
      matches(filters.priorities, task.priority) &&
      matches(filters.types, task.type) &&
      matches(filters.projects, task.project) &&
      matches(filters.goals, task.goal),
  );
}

/**
 * The types the Type filter offers: the workspace's own taxonomy, then any
 * type a loaded task carries that the taxonomy no longer lists.
 *
 * It was built from the loaded tasks alone, which the server had already
 * filtered by type, so after picking Bug the menu offered only Bug, and a type
 * no task had yet never appeared.
 */
export function filterableTypes(workspaceTypes: readonly { id: string }[], tasks: readonly Task[]): string[] {
  const ids = workspaceTypes.map((type) => type.id);
  const extra = Array.from(new Set(tasks.map((task) => task.type)))
    .filter((type) => type && !ids.includes(type))
    .sort((a, b) => a.localeCompare(b));
  return [...ids, ...extra];
}
