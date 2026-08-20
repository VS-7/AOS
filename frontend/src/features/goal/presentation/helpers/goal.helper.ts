import { GOAL_STATUS_CONFIG, GOAL_STATUS_ORDER } from "@/features/goal/presentation/consts/goal";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";

/**
 * @class GoalHelper
 * @description Provides standardized access to goal configurations including labels, icons, and colors.
 */
export class GoalHelper {
  /**
   * @method getStatus
   * @description Retrieves the configuration object for a specific goal status.
   * @param {Goal["status"]} status - The goal status.
   * @returns {Object} The status configuration.
   */
  public static getStatus(status: Goal["status"]) {
    return GOAL_STATUS_CONFIG[status];
  }

  /**
   * @method groupByStatus
   * @description Groups an array of goals by their status, following the predefined status order.
   * @param {Goal[]} goals - The array of goals to group.
   * @returns {Record<Goal["status"], Goal[]>} A record containing grouped goals.
   */
  public static groupByStatus(goals: Goal[]) {
    return GOAL_STATUS_ORDER.reduce(
      (acc, status) => {
        const items = goals.filter((g) => g.status === status);
        if (items.length > 0) acc[status] = items;
        return acc;
      },
      {} as Record<Goal["status"], Goal[]>,
    );
  }

  /**
   * @method formatDeadline
   * @description Formats an ISO deadline string into a readable date.
   * @param {string | undefined} isoString - The ISO timestamp.
   * @returns {string | null} The formatted date or null.
   */
  public static formatDeadline(isoString?: string): string | null {
    if (!isoString) return null;
    const date = new Date(isoString);
    return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
  }

  /**
   * @method isOverdue
   * @description Checks if a goal's deadline has passed.
   * @param {string | undefined} isoString - The ISO deadline timestamp.
   * @returns {boolean} True if the deadline is in the past.
   */
  public static isOverdue(isoString?: string): boolean {
    if (!isoString) return false;
    return new Date(isoString) < new Date();
  }
}
