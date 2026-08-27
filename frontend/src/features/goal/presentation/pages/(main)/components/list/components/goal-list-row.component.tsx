import React from "react";
import { motion } from "framer-motion";
import { Link, useRouter } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import { GoalHelper } from "@/features/goal/presentation/helpers/goal.helper";
import {
  GOAL_STATUS_CONFIG,
  GOAL_PRIORITY_CONFIG,
  goalPriorityConfig,
} from "@/features/goal/presentation/consts/goal";
import { aos } from "@/app/aos";
import { toast } from "sonner";
import { CalendarDays, MoreHorizontal, Trash2, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { SetPriorityDropdown } from "@/features/task/presentation/components/dropdowns/set-priority.dropdown";
import { t } from "@/lib/i18n";

interface GoalListRowProps {
  goal: Goal;
}

export function GoalListRow({ goal }: GoalListRowProps) {
  const router = useRouter();
  const status = GoalHelper.getStatus(goal.status);
  const StatusIcon = status.icon;
  const priority = goalPriorityConfig(goal.priority);
  const PriorityIcon = priority.icon;
  const deadlineFormatted = GoalHelper.formatDeadline(goal.deadline);
  const isOverdue = GoalHelper.isOverdue(goal.deadline);

  const handlePriorityChange = async (priority: Goal["priority"]) => {
    try {
      await aos.client.goal.update.mutate({
        params: { goal: goal.id },
        body: { priority },
      });
      toast.success(
        `Priority updated to ${goalPriorityConfig(priority).label}`,
      );
      router.invalidate();
    } catch {
      toast.error(t("Failed to update priority"));
    }
  };

  const handleStatusChange = async (status: Goal["status"]) => {
    try {
      await aos.client.goal.update.mutate({
        params: { goal: goal.id },
        body: { status },
      });
      toast.success(`Moved to ${GoalHelper.getStatus(status).label}`);
      router.invalidate();
    } catch {
      toast.error(t("Failed to update status"));
    }
  };

  const handleDelete = async () => {
    try {
      await aos.client.goal.delete.mutate({ params: { goal: goal.id } });
      toast.success(`Goal ${goal.id} deleted`);
      router.invalidate();
    } catch {
      toast.error(t("Failed to delete goal"));
    }
  };

  const handleCopyIdentifier = () => {
    navigator.clipboard.writeText(goal.id);
    toast.success(`${goal.id} copied`);
  };

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -4, transition: { duration: 0.15 } }}
      transition={{ duration: 0.2, ease: "easeOut" }}
      className="grid min-h-11 w-full grid-cols-[auto_auto_auto_1fr_auto_auto] items-center gap-2 px-3 py-2 transition-colors hover:border-input hover:bg-accent/40"
    >
      {/* Priority Dropdown */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center justify-center rounded p-1 hover:bg-accent">
            <PriorityIcon className={`size-3.5 ${priority.colorClass}`} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetPriorityDropdown
            currentPriority={goal.priority}
            onPriorityChange={handlePriorityChange}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Goal ID */}
      <Link to="/goals/$id" params={{ id: goal.id }}>
        <span className="shrink-0 font-mono text-sm text-muted-foreground">
          {goal.id}
        </span>
      </Link>

      {/* Status Dropdown */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center gap-1 rounded p-1 hover:bg-accent">
            <StatusIcon className={`size-3.5 ${status.color}`} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {Object.entries(GOAL_STATUS_CONFIG).map(([s, cfg]) => {
            const Icon = cfg.icon;
            return (
              <DropdownMenuItem
                key={s}
                onClick={() => handleStatusChange(s as Goal["status"])}
                className="flex items-center gap-2"
              >
                <Icon className={`size-4 ${cfg.color}`} />
                <span>{cfg.label}</span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Title */}
      <Link
        to="/goals/$id"
        params={{ id: goal.id }}
        className="flex flex-col min-w-0"
      >
        <span className="truncate text-sm font-medium">{goal.title}</span>
      </Link>

      {/* Deadline */}
      {deadlineFormatted && (
        <span
          className={cn(
            "flex items-center gap-1 shrink-0 text-xs",
            isOverdue ? "text-red-500" : "text-muted-foreground",
          )}
        >
          <CalendarDays className="size-3" />
          {deadlineFormatted}
        </span>
      )}

      {/* Actions */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="size-7 rounded-md">
            <MoreHorizontal className="size-3.5 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onClick={handleCopyIdentifier}
            className="flex items-center gap-2"
          >
            <Copy className="size-3.5" />
            {t("Copy ID")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={handleDelete}
            className="flex items-center gap-2 text-red-500 focus:text-red-500"
          >
            <Trash2 className="size-3.5" />
            {t("Delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </motion.div>
  );
}
