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

interface AgentTasksTabProps {
  agent: Agent;
}

export function AgentTasksTab({ agent }: AgentTasksTabProps) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    let isMounted = true;
    setIsLoading(true);

    aos.client.task.list
      .query({ query: { limit: "200" } })
      .then((response) => {
        if (!isMounted) return;
        const allTasks: Task[] = response.data?.tasks ?? [];
        const selectedAgentId = agent.id.toLowerCase();
        const filtered = allTasks.filter((task) => (task.assigned ?? "").toLowerCase() === selectedAgentId);
        setTasks(filtered);
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
        <span className="text-xs uppercase tracking-wide text-muted-foreground">Assigned tasks</span>
        <span className="text-xs text-muted-foreground">{tasks.length} total</span>
      </div>

      {!isLoading && tasks.length === 0 && (
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
            <AnimatedEmptyState.Title>No tasks assigned</AnimatedEmptyState.Title>
            <AnimatedEmptyState.Description>
              No tasks are currently assigned to this agent.
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
