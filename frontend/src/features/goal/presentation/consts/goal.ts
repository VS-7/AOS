import {
  Target,
  CheckCircle2,
  CircleX,
  CircleDashed,
  SignalZero,
  SignalLow,
  SignalMedium,
  SignalHigh,
  FlagIcon,
} from "lucide-react";
import type {
  FractalGoal,
  FractalGoalPriority,
} from "@/features/goal/interfaces/goal.interfaces";

export const GOAL_STATUS_CONFIG: Record<
  FractalGoal["status"],
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
};

export const GOAL_STATUS_ORDER: FractalGoal["status"][] = [
  "active",
  "achieved",
  "abandoned",
];

export const GOAL_PRIORITY_CONFIG: Record<
  FractalGoalPriority,
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

export const GOAL_PRIORITY_ORDER: FractalGoalPriority[] = [
  "no_priority",
  "urgent",
  "high",
  "medium",
  "low",
];
