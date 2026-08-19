import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { CalendarClock, Plus } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { aos } from "@/app/aos";
import type { FractalAgent } from "@/features/agent/interfaces/agent.interfaces";
import type { FractalRoutine } from "@/features/routine/interfaces/routine.interfaces";
import { ROUTINE_STATUS_ORDER } from "@/features/routine/presentation/consts/routine";
import { FractalRoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { RoutineListSection } from "@/features/routine/presentation/pages/(main)/components/list/components/routine-list-section.component";

interface AgentRoutinesTabProps {
  agent: FractalAgent;
}

export function AgentRoutinesTab({ agent }: AgentRoutinesTabProps) {
  const navigate = useNavigate();
  const [routines, setRoutines] = useState<FractalRoutine[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    let isMounted = true;
    setIsLoading(true);

    aos.client.routine.list
      .query({ query: { agent: agent.id, limit: "200" } })
      .then((response) => {
        if (!isMounted) return;
        setRoutines(response.data?.routines ?? []);
      })
      .finally(() => {
        if (!isMounted) return;
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [agent.id]);

  const grouped = useMemo(() => FractalRoutineHelper.groupByStatus(routines), [routines]);

  return (
    <div className="container max-w-6xl mx-auto px-6 py-6 pb-10 space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div className="space-y-0.5">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">Agent routines</p>
          <p className="text-xs text-muted-foreground">{routines.length} total</p>
        </div>

        <Button type="button" size="sm" onClick={() => navigate({ to: "/routines/$id", params: { id: "new" } })}>
          <Plus className="size-4" />
          New Routine
        </Button>
      </div>

      {isLoading && (
        <div className="space-y-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      )}

      {!isLoading && routines.length === 0 && (
        <AnimatedEmptyState className="border-none shadow-none py-12">
          <AnimatedEmptyState.Carousel>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                <CalendarClock className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-2 w-24 rounded-md bg-muted" />
                <div className="h-2 w-16 rounded-md bg-muted/50" />
              </div>
            </div>
          </AnimatedEmptyState.Carousel>
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>No routines configured</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              This agent does not have routines yet.
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      )}

      {!isLoading && routines.length > 0 && (
        <div className="gap-4">
          {ROUTINE_STATUS_ORDER.map((status) => (
            <RoutineListSection key={status} status={status} routines={grouped[status] || []} />
          ))}
        </div>
      )}
    </div>
  );
}
