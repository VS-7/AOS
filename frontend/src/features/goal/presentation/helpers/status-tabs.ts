import * as React from "react";

/**
 * The status sub-tabs on a goal's Tasks section and a project's Tasks and
 * Goals tabs.
 *
 * They are icon-only, show only the selected one's label, and each opened on
 * a fixed status ("Todo", "Active"). A project whose only task was stopped
 * said "No tasks in this status." under a KPI of one task, with nothing to
 * say which of eight icons held it. They open on the first status that has
 * something now, and every icon carries its count.
 */

/** How many items sit under each status of `order`. */
export function countByStatus<S extends string>(
  order: readonly S[],
  items: ReadonlyArray<{ status: string }>,
): Record<S, number> {
  const counts = Object.fromEntries(order.map((status) => [status, 0])) as Record<S, number>;
  for (const item of items) {
    if (item.status in counts) counts[item.status as S] += 1;
  }
  return counts;
}

/** The tab to show: the one chosen, else the first that has items. */
export function openingStatus<S extends string>(
  order: readonly S[],
  counts: Record<S, number>,
  chosen: S | null,
): S {
  if (chosen) return chosen;
  return order.find((status) => counts[status] > 0) ?? order[0];
}

/**
 * The selected status and the counts, for items that may still be loading:
 * until the person picks a tab, the selection follows the data.
 */
export function useStatusTabs<S extends string>(
  order: readonly S[],
  items: ReadonlyArray<{ status: string }>,
) {
  const [chosen, setChosen] = React.useState<S | null>(null);
  const counts = React.useMemo(() => countByStatus(order, items), [order, items]);
  const selected = openingStatus(order, counts, chosen);
  return { selected, select: setChosen, counts };
}
