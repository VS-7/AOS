import React, { useCallback } from "react";
import { Link, useRouter } from "@tanstack/react-router";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Avatar,
  AvatarAgentFallback,
} from "@/components/ui/avatar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { aos } from "@/app/aos";
import { cn, timeAgo } from "@/lib/utils";
import { toast } from "sonner";
import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineActionsDropdown,
  SetRoutineAgentDropdown,
  SetRoutineStatusDropdown,
} from "@/features/routine/presentation/components/dropdowns";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";

interface RoutineListRowProps {
  routine: Routine;
}

export const RoutineListRow = React.memo(function RoutineListRow({
  routine,
}: RoutineListRowProps) {
  const router = useRouter();
  const agents = aos.stores.agent.useState((state) => state.items);

  const status = RoutineHelper.getStatus(routine.status);
  const StatusIcon = status.icon;
  const agentLabel = RoutineHelper.getAgentLabel(routine.agent, agents);
  const triggersLabel = RoutineHelper.getTriggersInlineLabel(
    routine.triggers,
  );
  const updatedLabel = timeAgo(routine.updatedAt);

  const handleStatusChange = useCallback(
    async (nextStatus: Routine["status"]) => {
      try {
        await aos.client.routine.update.mutate({
          params: { routine: routine.id },
          body: { status: nextStatus },
        });
        toast.success(
          `Status updated to ${RoutineHelper.getStatus(nextStatus).label}`,
        );
        router.invalidate();
      } catch {
        toast.error("Failed to update status");
      }
    },
    [routine.id, router],
  );

  const handleAgentChange = useCallback(
    async (agent: string) => {
      try {
        await aos.client.routine.update.mutate({
          params: { routine: routine.id },
          body: { agent },
        });
        toast.success(
          `Assigned to ${RoutineHelper.getAgentLabel(agent, agents)}`,
        );
        router.invalidate();
      } catch {
        toast.error("Failed to update agent");
      }
    },
    [agents, routine.id, router],
  );

  const handleFire = useCallback(async () => {
    try {
      const result = await aos.client.routine.fire.mutate({
        params: { routine: routine.id },
        query: {},
        body: {},
      });

      if (result?.error) {
        toast.error("Failed to start routine");
        return;
      }

      const executionCount = result.data?.executions?.length ?? 1;
      toast.success(
        executionCount > 1
          ? `Routine started for ${executionCount} agents`
          : "Routine started",
      );
      router.invalidate();
    } catch {
      toast.error("Failed to start routine");
    }
  }, [routine.id, router]);

  const handleCopyIdentifier = useCallback(() => {
    navigator.clipboard.writeText(routine.id);
    toast.success(`${routine.id} copied`);
  }, [routine.id]);

  const handleDelete = useCallback(async () => {
    const confirmed = window.confirm(
      `Delete routine "${routine.name}"? This cannot be undone.`,
    );
    if (!confirmed) return;

    try {
      await aos.client.routine.delete.mutate({
        params: { routine: routine.id },
      });
      toast.success(`Routine ${routine.id} deleted`);
      router.invalidate();
    } catch {
      toast.error("Failed to delete routine");
    }
  }, [routine.id, routine.name, router]);

  return (
    <div
      className={cn(
        "grid min-h-11 w-full grid-cols-[auto_auto_minmax(0,1fr)_minmax(0,10rem)_auto_auto] items-center gap-2 px-3 py-2 transition-colors hover:bg-accent/40",
        routine.status === "disabled" && "opacity-70",
      )}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex items-center justify-center rounded p-1 hover:bg-accent"
            aria-label={`Change status for ${routine.name}`}
          >
            <StatusIcon className={`size-3.5 ${status.color}`} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetRoutineStatusDropdown
            currentStatus={routine.status}
            onStatusChange={handleStatusChange}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <Link to="/routines/$id" params={{ id: routine.id }}>
        <span className="max-w-24 truncate font-mono text-xs text-muted-foreground">
          {routine.id}
        </span>
      </Link>

      <Link to="/routines/$id" params={{ id: routine.id }} className="min-w-0">
        <span className="block truncate text-sm font-medium">{routine.name}</span>
      </Link>

      <Tooltip>
        <TooltipTrigger asChild>
          <span className="hidden min-w-0 truncate text-xs text-muted-foreground sm:block">
            {triggersLabel}
            <span className="mx-1 text-border">·</span>
            {updatedLabel}
          </span>
        </TooltipTrigger>
        <TooltipContent sideOffset={6}>
          {triggersLabel} · Updated {updatedLabel}
        </TooltipContent>
      </Tooltip>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex items-center justify-center rounded p-1 hover:bg-accent"
            aria-label={`Change agent for ${routine.name}`}
          >
            <Avatar size="sm">
              <AvatarAgentFallback
                size={26}
                name={agentLabel.toLowerCase()}
              />
            </Avatar>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64">
          <SetRoutineAgentDropdown
            currentAgent={routine.agent}
            onAgentChange={handleAgentChange}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <RoutineActionsDropdown
        routine={routine}
        onFire={handleFire}
        onCopyIdentifier={handleCopyIdentifier}
        onDelete={handleDelete}
      />
    </div>
  );
});
