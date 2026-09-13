import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { TabsSubtle, TabsSubtleItem } from "@/components/ui/tabs-subtle";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import {
  TASK_STATUS_CONFIG,
  TASK_STATUS_ORDER,
} from "@/features/task/presentation/consts/task";
import { TaskListRow } from "@/features/task/presentation/pages/(main)/components/list/components/task-list-row.component";
import { aos } from "@/app/aos";
import { Plus } from "lucide-react";
import { useDelayedLoading } from "@/hooks/use-delayed-loading.hook";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import { useStatusTabs } from "@/features/goal/presentation/helpers/status-tabs";
import { useTranslation } from "@/lib/i18n";

interface ProjectTasksTabProps {
  project: Project;
}

function TasksListSkeleton() {
  return (
    <div className="rounded-md border bg-card divide-y overflow-hidden">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="grid min-h-11 w-full grid-cols-[auto_auto_auto_1fr_auto_auto_auto] items-center gap-2 px-3 py-2"
        >
          <Skeleton className="size-3.5 rounded" />
          <Skeleton className="h-4 w-12 rounded" />
          <Skeleton className="size-3.5 rounded" />
          <Skeleton className="h-4 w-48 rounded" />
          <Skeleton className="h-5 w-16 rounded" />
          <Skeleton className="size-6 rounded-full" />
          <Skeleton className="size-6 rounded" />
        </div>
      ))}
    </div>
  );
}

interface SectionProps {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}

function TasksSection({ title, subtitle, action, children }: SectionProps) {
  return (
    <section>
      <header className="flex items-center justify-between gap-4 py-4">
        <div className="space-y-0.5">
          <h2 className="text-md font-semibold tracking-tight">{title}</h2>
          {subtitle && (
            <p className="text-xs text-muted-foreground">{subtitle}</p>
          )}
        </div>
        {action}
      </header>
      {children}
    </section>
  );
}

export function ProjectTasksTab({ project }: ProjectTasksTabProps) {
  // Subscribing re-renders the tab labels when the language changes; they
  // used to be spread out of TASK_STATUS_CONFIG once, at module load.
  const { t } = useTranslation();

  const taskQuery = aos.client.task.list.useQuery({
    query: { project: project.id, limit: 200 },
    staleTime: 5 * 60 * 1000,
  });

  const tasks: Task[] = useMemo(() => taskQuery.data?.tasks ?? [], [taskQuery.data]);
  const isLoading = useDelayedLoading(taskQuery.isLoading);
  // Opens on the first status with tasks: it opened on "Todo" and said "No
  // tasks in this status." while the project's one task sat behind "Stopped".
  const { selected: selectedStatus, select, counts } = useStatusTabs(TASK_STATUS_ORDER, tasks);

  const filteredTasks = useMemo(
    () => tasks.filter((t) => t.status === selectedStatus),
    [tasks, selectedStatus],
  );

  return (
    <div className="container mx-auto max-w-3xl py-6 pb-12 space-y-6">
      <TasksSection
        title={t("Tasks")}
        action={
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => (aos.triggers as { dispatch: (id: string, input?: unknown) => Promise<unknown> }).dispatch("tasks.new")}
          >
            <Plus className="size-4" />
            {t("New Task")}
          </Button>
        }
      >
        <TabsSubtle
          activeLabel
          selectedIndex={TASK_STATUS_ORDER.indexOf(selectedStatus)}
          onSelect={(index) => select(TASK_STATUS_ORDER[index])}
        >
          {TASK_STATUS_ORDER.map((status, index) => (
            <TabsSubtleItem
              key={status}
              index={index}
              label={TASK_STATUS_CONFIG[status].label}
              icon={TASK_STATUS_CONFIG[status].icon}
              count={counts[status] || undefined}
            />
          ))}
        </TabsSubtle>

        <div className="mt-3">
          {isLoading ? (
            <TasksListSkeleton />
          ) : filteredTasks.length === 0 ? (
            <div className="rounded-md border-2 border-dotted h-12 flex items-center justify-center w-full">
              <span className="text-xs text-muted-foreground/60">
                {t("No tasks in this status.")}
              </span>
            </div>
          ) : (
            <div className="rounded-md border bg-card divide-y overflow-hidden">
              {filteredTasks.map((task) => (
                <TaskListRow key={task.id} task={task} />
              ))}
            </div>
          )}
        </div>
      </TasksSection>
    </div>
  );
}
