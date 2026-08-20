import { TASK_STATUS_CONFIG, TASK_PRIORITY_CONFIG, TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import { TODO_STATUS_CONFIG } from "@/features/task/presentation/consts/todo";
import type { Task, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import type { Todo } from "@/features/task/interfaces/todo.interfaces";

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
