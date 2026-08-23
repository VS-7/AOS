import {
  Target,
  CheckCircle2,
  CircleX,
  CirclePause,
  CircleDashed,
  SignalZero,
  SignalLow,
  SignalMedium,
  SignalHigh,
  FlagIcon,
} from "lucide-react";
import type {
  Goal,
  GoalPriority,
} from "@/features/goal/interfaces/goal.interfaces";

export const GOAL_STATUS_CONFIG: Record<
  Goal["status"],
  { label: string; icon: any; color: string; badgeClass: string }
> = {
  active: {
    label: "Active",
    icon: CircleDashed,
    color: "text-primary",
    badgeClass: "bg-primary/10 text-primary border-primary/20",
  },
  achieved: {
    label: "Achieved",
    icon: CheckCircle2,
    color: "text-success",
    badgeClass: "bg-success/10 text-success border-success/20",
  },
  abandoned: {
    label: "Abandoned",
    icon: CircleX,
    color: "text-muted-foreground",
    badgeClass: "bg-muted text-muted-foreground border-muted",
  },
  // Go's `goal.Status` union has four members, not three
  // (`internal/domain/goal/entity.go`: active, achieved, abandoned,
  // *paused*). Without this entry every lookup of a paused goal returned
  // `undefined` and the row that rendered it threw on `.icon`.
  paused: {
    label: "Paused",
    icon: CirclePause,
    color: "text-muted-foreground",
    badgeClass: "bg-muted text-muted-foreground border-muted",
  },
};

export const GOAL_STATUS_ORDER: Goal["status"][] = [
  "active",
  "paused",
  "achieved",
  "abandoned",
];

export const GOAL_PRIORITY_CONFIG: Record<
  GoalPriority,
  { label: string; icon: any; colorClass: string }
> = {
  no_priority: {
    label: "No Priority",
    icon: SignalZero,
    colorClass: "text-muted-foreground/70",
  },
  low: {
    label: "Low",
    icon: SignalLow,
    colorClass: "text-muted-foreground/70",
  },
  medium: {
    label: "Medium",
    icon: SignalMedium,
    colorClass: "text-muted-foreground/70",
  },
  high: { label: "High", icon: SignalHigh, colorClass: "text-yellow-500" },
  urgent: { label: "Urgent", icon: FlagIcon, colorClass: "text-red-500" },
};

export const GOAL_PRIORITY_ORDER: GoalPriority[] = [
  "no_priority",
  "urgent",
  "high",
  "medium",
  "low",
];

/**
 * Total lookups over the two config records above — they never return
 * `undefined`, whatever the backend sent.
 *
 * These exist because indexing the records directly is only safe when the
 * key is known to be one this build declares, and a value read off a Go
 * `Goal` never is:
 *
 * - `priority` does not exist on Go's `Goal`
 *   (`internal/domain/goal/entity.go` — no such field, and
 *   `goal.CreateInput`/`UpdateInput` have no way to set one). Every goal
 *   the daemon returns therefore has `priority === undefined`, so
 *   `GOAL_PRIORITY_CONFIG[goal.priority]` was `undefined` and reading
 *   `.icon` off it threw — taking down the whole Goals list *and* the
 *   home screen that embeds it, on every single goal, via the router's
 *   error boundary. That is the "Cannot read properties of undefined
 *   (reading 'icon')" screen.
 * - `status` can be `paused`, which this config was missing until now.
 *   Adding the entry fixes today's four values; going through this
 *   function means a fifth one the daemon grows later degrades to a
 *   neutral row instead of a blank screen.
 *
 * Priority is presentation-only against this backend: the picker still
 * works in the session, and the value is dropped by the daemon rather
 * than persisted, because there is no field to persist it in.
 */
export function goalStatusConfig(status: Goal["status"] | undefined) {
  return (
    (status && GOAL_STATUS_CONFIG[status]) ?? GOAL_STATUS_CONFIG.active
  );
}

export function goalPriorityConfig(priority: GoalPriority | undefined) {
  return (
    (priority && GOAL_PRIORITY_CONFIG[priority]) ??
    GOAL_PRIORITY_CONFIG.no_priority
  );
}
