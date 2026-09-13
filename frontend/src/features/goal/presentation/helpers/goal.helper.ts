import { GOAL_STATUS_ORDER, goalStatusConfig } from "@/features/goal/presentation/consts/goal";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import { formatDeadline, isDeadlineOverdue } from "./goal-deadline";

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
  public static getStatus(status: Goal["status"] | undefined) {
    // Total by way of `goalStatusConfig` — see its doc comment for why a
    // status read off a Go `Goal` cannot be assumed to be one this build
    // declares.
    return goalStatusConfig(status);
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
   * @description The deadline's calendar day, in the interface's language —
   * see `goal-deadline.ts` for why a picked day is read in UTC.
   * @param {string | undefined} isoString - The ISO timestamp.
   * @returns {string | null} The formatted date or null.
   */
  public static formatDeadline(isoString?: string): string | null {
    return formatDeadline(isoString);
  }

  /**
   * @method isOverdue
   * @description Checks if a goal's deadline has passed.
   * @param {string | undefined} isoString - The ISO deadline timestamp.
   * @returns {boolean} True if the deadline is in the past.
   */
  public static isOverdue(isoString?: string): boolean {
    return isDeadlineOverdue(isoString);
  }
}
