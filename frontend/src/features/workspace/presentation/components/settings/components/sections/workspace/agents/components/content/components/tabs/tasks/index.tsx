import { useEffect, useMemo, useState } from "react";
import { ListChecks } from "lucide-react";
import { AnimatedEmptyState } from "@/components/ui/animated-empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import type { Agent } from "@/features/agent/interfaces/agent.interfaces";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { TASK_STATUS_ORDER } from "@/features/task/presentation/consts/task";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { aos } from "@/app/aos";
import { AgentTaskStatusSection } from "./components/task-status-section";
import { t } from "@/lib/i18n";

interface AgentTasksTabProps {
  agent: Agent;
}

export function AgentTasksTab({ agent }: AgentTasksTabProps) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;
    setIsLoading(true);
    setLoadError(null);

    // Asked of the daemon, which filters by owner. This read the first 200
    // tasks of the workspace and filtered them here, so an agent whose tasks
    // were not among those 200 showed none.
    aos.client.task.list
      .query({ query: { assigned: agent.id } })
      .then((response) => {
        if (!isMounted) return;
        if (response.error) {
          setLoadError(response.error.message ?? t("Could not load this agent's tasks."));
          setTasks([]);
          return;
        }
        setTasks(response.data?.tasks ?? []);
      })
      .finally(() => {
        if (!isMounted) return;
        setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [agent.id]);

  const grouped = useMemo(() => TaskHelper.groupByStatus(tasks), [tasks]);

  return (
    <div className="container max-w-6xl mx-auto px-6 py-6 pb-10">
      <div className="flex items-center justify-between mb-3 px-2">
        <span className="text-xs uppercase tracking-wide text-muted-foreground">{t("Assigned tasks")}</span>
        <span className="text-xs text-muted-foreground">
          {t("{{count}} total", { count: tasks.length })}
        </span>
      </div>

      {!isLoading && loadError ? (
        <p className="px-2 text-sm text-destructive" role="alert">
          {loadError}
        </p>
      ) : null}

      {!isLoading && !loadError && tasks.length === 0 && (
        <AnimatedEmptyState className="border-none shadow-none py-12">
          <AnimatedEmptyState.Carousel>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-md bg-muted/50">
                <ListChecks className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-2 w-24 rounded-md bg-muted" />
                <div className="h-2 w-16 rounded-md bg-muted/50" />
              </div>
            </div>
          </AnimatedEmptyState.Carousel>
          <AnimatedEmptyState.Content>
            <AnimatedEmptyState.Title>{t("No tasks assigned")}</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              {t("No tasks are currently assigned to this agent.")}
            </AnimatedEmptyState.Description>
          </AnimatedEmptyState.Content>
        </AnimatedEmptyState>
      )}

      {!isLoading && tasks.length > 0 && (
        <div className="gap-4">
          {TASK_STATUS_ORDER.map((status) => (
            <AgentTaskStatusSection key={status} status={status} tasks={grouped[status] || []} />
          ))}
        </div>
      )}
    </div>
  );
}
