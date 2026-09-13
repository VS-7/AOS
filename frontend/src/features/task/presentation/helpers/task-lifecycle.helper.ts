import { TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import type { Task, TaskStatus } from "@/features/task/interfaces/task.interfaces";

type LifecycleTask = Pick<Task, "status" | "nextStates">;

/**
 * The statuses this task can be moved to, as the daemon publishes them.
 *
 * Every picker used to list all eight statuses and every kanban column took a
 * drop, so the only description of the lifecycle the person ever saw was
 * AOS_TASK_INVALID_TRANSITION after the fact. `tasks_get`/`tasks_list` carry
 * `nextStates`, read from the one table that decides moves
 * (internal/domain/task/state.go), so nothing here keeps a copy to drift.
 *
 * A view without the field comes from a daemon that predates it: every other
 * status is offered, as before, and the daemon's refusal is what is shown.
 */
export function allowedMoves(task: LifecycleTask): TaskStatus[] {
  if (Array.isArray(task.nextStates)) return task.nextStates;
  return TASK_STATUS_ORDER.filter((status) => status !== task.status);
}

/** Whether the lifecycle lets this task move to `status`. Staying put is not a move. */
export function canMoveTo(task: LifecycleTask, status: TaskStatus): boolean {
  return status !== task.status && allowedMoves(task).includes(status);
}

export type TaskNextStepKind = "start" | "resume" | "continue" | "approve" | "backlog" | "todo";

export interface TaskNextStep {
  kind: TaskNextStepKind;
  status: TaskStatus;
}

/**
 * The one move the task page's header offers as its button.
 *
 * Start used to show on suggestion, backlog and planning tasks, which cannot
 * reach in_progress, and Continue on a task already in progress, which
 * re-sent the status it had and toasted "Started". The button is now the next
 * legal step from where the task is: work starts from todo and resumes from
 * stopped; a task earlier in the lifecycle is moved one step on; a review
 * whose plan is settled is approved, and one whose plan was reopened goes back
 * to work.
 */
export function nextStep(task: LifecycleTask, planSettled: boolean): TaskNextStep | null {
  const pick = (kind: TaskNextStepKind, status: TaskStatus): TaskNextStep | null =>
    canMoveTo(task, status) ? { kind, status } : null;

  switch (task.status) {
    case "suggestion":
      return pick("backlog", "backlog");
    case "backlog":
    case "planning":
      return pick("todo", "todo");
    case "todo":
      return pick("start", "in_progress");
    case "stopped":
      return pick("resume", "in_progress");
    case "in_review":
      return planSettled ? pick("approve", "finished") : pick("continue", "in_progress");
    default:
      return null;
  }
}
