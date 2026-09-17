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
import { cn } from "@/lib/utils";
import { useAlert } from "@/components/ui/alert-provider";
import { toast } from "sonner";
import type {
  Routine,
  RoutineStatus,
} from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineActionsDropdown,
  SetRoutineStatusDropdown,
} from "@/features/routine/presentation/components/dropdowns";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";
import { describeFireFailure } from "@/features/routine/presentation/helpers/routine-fire.helper";
import { useRoutinesContext } from "@/features/routine/presentation/pages/(main)/context";

interface RoutineListRowProps {
  routine: Routine;
}

export const RoutineListRow = React.memo(function RoutineListRow({
  routine,
}: RoutineListRowProps) {
  const router = useRouter();
  const { confirm } = useAlert();
  const { activityEvents } = useRoutinesContext();
  const agents = aos.stores.agent.useState((state) => state.items);

  const status = RoutineHelper.getStatus(routine.status);
  const StatusIcon = status.icon;
  const agentLabel = RoutineHelper.getAgentLabel(routine.agent, agents);
  const triggersLabel = RoutineHelper.getTriggersInlineLabel(
    routine.triggers,
    activityEvents,
  );
  const updatedLabel = RoutineHelper.relativeTime(routine.updatedAt);

  const handleStatusChange = useCallback(
    async (nextStatus: RoutineStatus) => {
      if (nextStatus === routine.status) return;
      try {
        await aos.client.routine.update.mutateOrThrow({
          params: { routine: routine.id },
          body: { status: nextStatus },
        });
        toast.success(
          t("Status updated to {{status}}", {
            status: RoutineHelper.getStatus(nextStatus).label,
          }),
        );
        router.invalidate();
      } catch (error) {
        toast.error(t("Failed to update status"), { description: errorMessage(error) });
      }
    },
    [routine.id, routine.status, router],
  );

  const handleFire = useCallback(async () => {
    try {
      await aos.client.routine.fire.mutateOrThrow({
        params: { routine: routine.id },
        query: {},
        body: {},
      });
      toast.success(t("The routine ran."));
      router.invalidate();
    } catch (error) {
      const failure = await describeFireFailure(routine.id, error);
      toast.error(failure.title, { description: failure.description });
      router.invalidate();
    }
  }, [routine.id, router]);

  const handleCopyIdentifier = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(routine.id);
      toast.success(t("Identifier copied."));
    } catch (error) {
      toast.error(t("Failed to copy"), { description: errorMessage(error) });
    }
  }, [routine.id]);

  const handleDelete = useCallback(async () => {
    // See channel-item: `window.confirm` answers false without asking inside
    // the desktop window, so this used the application's dialog instead.
    const confirmed = await confirm({
      title: t("Delete routine \"{{name}}\"?", { name: routine.name }),
      description: t("This cannot be undone."),
      confirmText: t("Delete"),
      variant: "destructive",
    });
    if (!confirmed) return;

    try {
      await aos.client.routine.delete.mutateOrThrow({
        params: { routine: routine.id },
      });
      toast.success(t("Routine deleted."));
      router.invalidate();
    } catch (error) {
      toast.error(t("Failed to delete routine"), { description: errorMessage(error) });
    }
  }, [confirm, routine.id, routine.name, router]);

  return (
    <div
      className={cn(
        "grid min-h-11 w-full grid-cols-[auto_auto_minmax(0,1fr)_minmax(0,14rem)_auto_auto] items-center gap-2 px-3 py-2 transition-colors hover:bg-accent/40",
        routine.status === "disabled" && "opacity-70",
      )}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex items-center justify-center rounded p-1 hover:bg-accent"
            aria-label={t("Change status for {{name}}", { name: routine.name })}
          >
            <StatusIcon className={`size-3.5 ${status.color}`} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <SetRoutineStatusDropdown
            currentStatus={routine.status}
            onStatusChange={(next) => void handleStatusChange(next)}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      {/* A short prefix, as a block: the whole UUID in an inline span ignored
          its truncation and took a third of the row. */}
      <Link
        to="/routines/$id"
        params={{ id: routine.id }}
        className="block w-16 truncate font-mono text-xs text-muted-foreground"
        title={routine.id}
      >
        {RoutineHelper.shortId(routine.id)}
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
          {t("{{triggers}} · Updated {{when}}", { triggers: triggersLabel, when: updatedLabel })}
        </TooltipContent>
      </Tooltip>

      {/* The owner, shown rather than offered: Go cannot move a routine to
          another agent, and every choice made here was refused. */}
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="flex items-center justify-center p-1">
            <Avatar size="sm">
              <AvatarAgentFallback size={26} name={agentLabel.toLowerCase()} />
            </Avatar>
          </span>
        </TooltipTrigger>
        <TooltipContent sideOffset={6}>{agentLabel}</TooltipContent>
      </Tooltip>

      <RoutineActionsDropdown
        routine={routine}
        onFire={() => void handleFire()}
        onCopyIdentifier={() => void handleCopyIdentifier()}
        onDelete={() => void handleDelete()}
      />
    </div>
  );
});
