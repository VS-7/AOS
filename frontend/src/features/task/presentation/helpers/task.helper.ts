import { TASK_STATUS_CONFIG, TASK_PRIORITY_CONFIG, TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import { TODO_STATUS_CONFIG } from "@/features/task/presentation/consts/todo";
import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";
import { getLocale } from "@/lib/i18n";

/** The branch prefix a workspace uses when it declares none (Go's task.DefaultBranchPrefix). */
const DEFAULT_BRANCH_PREFIX = "aos";

/**
 * @class TaskHelper
 * @description Provides standardized access to task and todo configurations including labels, icons, and colors.
 */
export class TaskHelper {
  /**
   * @method getStatus
   * @description Retrieves the configuration object for a specific task status.
   * @param {Task["status"]} status - The task status.
   * @returns {Object} The status configuration.
   */
  public static getStatus(status: Task["status"]) {
    return TASK_STATUS_CONFIG[status];
  }

  /**
   * @method getPriority
   * @description Retrieves the configuration object for a specific task priority.
   * @param {TaskPriority} priority - The task priority.
   * @returns {Object} The priority configuration.
   */
  public static getPriority(priority: TaskPriority) {
    return TASK_PRIORITY_CONFIG[priority];
  }

  /**
   * @method getTodoStatus
   * @description Retrieves the configuration object for a specific todo status.
   * @param {Todo["status"]} status - The todo status.
   * @returns {Object} The status configuration.
   */
  public static getTodoStatus(status: Todo["status"]) {
    return TODO_STATUS_CONFIG[status];
  }

  /**
   * The identifier a person reads: the first block of the UUID.
   *
   * Rows, cards and the page title printed all 36 characters in monospace,
   * which squeezed the name out of a row and wrapped a card's header over
   * three lines. The full id stays one click away (Copy task ID).
   */
  public static shortId(id: string): string {
    return id.split("-")[0] || id;
  }

  /**
   * The branch the task's checkout is on, or will be cut on.
   *
   * "Copy branch name" copied `<type>/<slug>`, a name the daemon never uses:
   * it cuts `<workspace prefix>/<slug>` (internal/domain/task/worktree.go's
   * BranchNameFor), unless the task records a branch of its own.
   */
  public static branchName(task: Pick<Task, "slug" | "id" | "worktree">, prefix?: string): string {
    if (task.worktree?.branch) return task.worktree.branch;
    return `${prefix?.trim() || DEFAULT_BRANCH_PREFIX}/${task.slug || task.id}`;
  }

  /**
   * An instant in the interface's language rather than the operating
   * system's, so an English window does not show "31 de ago. de 2026". The
   * time is left out when the instant is a local midnight, which is what a
   * date picked without a time is.
   */
  public static formatDate(value: string | undefined | null): string | null {
    if (!value) return null;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return null;
    const midnight = date.getHours() === 0 && date.getMinutes() === 0;
    return new Intl.DateTimeFormat(getLocale(), midnight ? { dateStyle: "medium" } : { dateStyle: "medium", timeStyle: "short" }).format(date);
  }

  /**
   * @method groupByStatus
   * @description Groups an array of tasks by their status, following the predefined status order.
   * @param {Task[]} tasks - The array of tasks to group.
   * @returns {Record<Task["status"], Task[]>} A record containing grouped tasks.
   */
  public static groupByStatus(tasks: Task[]) {
    return TASK_STATUS_ORDER.reduce(
      (acc, status) => {
        const items = tasks.filter((t) => t.status === status);
        if (items.length > 0) acc[status] = items;
        return acc;
      },
      {} as Record<Task["status"], Task[]>,
    );
  }
}
