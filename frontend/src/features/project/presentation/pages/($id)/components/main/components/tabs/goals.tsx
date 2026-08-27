import * as React from "react";
import { useNavigate, Link } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { TabsSubtle, TabsSubtleItem } from "@/components/ui/tabs-subtle";
import {
  GOAL_STATUS_CONFIG,
  goalStatusConfig,
  GOAL_STATUS_ORDER,
} from "@/features/goal/presentation/consts/goal";
import { GoalHelper } from "@/features/goal/presentation/helpers/goal.helper";
import { aos } from "@/app/aos";
import { Plus, ArrowUpRight, CalendarDays } from "lucide-react";
import { useDelayedLoading } from "@/hooks/use-delayed-loading.hook";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import type { Project } from "@/features/project/interfaces/project.interfaces";
import { t } from "@/lib/i18n";

interface ProjectGoalsTabProps {
  project: Project;
}

const GOAL_TABS = GOAL_STATUS_ORDER.map((status) => ({
  status,
  ...GOAL_STATUS_CONFIG[status],
}));

interface GoalRowProps {
  goal: Goal;
}

function GoalRow({ goal }: GoalRowProps) {
  const statusCfg = goalStatusConfig(goal.status);
  const StatusIcon = statusCfg.icon;
  const deadline = GoalHelper.formatDeadline(goal.deadline);
  const isOverdue = GoalHelper.isOverdue(goal.deadline);

  return (
    <Link
      to="/goals/$id"
      params={{ id: goal.id }}
      className="group flex items-center justify-between gap-4 px-4 py-3 transition hover:bg-accent/50"
    >
      <div className="flex items-center gap-3 min-w-0">
        <StatusIcon className={`size-4 shrink-0 ${statusCfg.color}`} />
        <div className="min-w-0">
          <div className="text-sm font-medium truncate">{goal.title}</div>
          {goal.description && (
            <div className="text-xs text-muted-foreground truncate mt-0.5">
              {goal.description}
            </div>
          )}
        </div>
      </div>
      <div className="flex items-center gap-3 shrink-0">
        {deadline && (
          <div
            className={`flex items-center gap-1 text-xs ${isOverdue ? "text-destructive" : "text-muted-foreground"}`}
          >
            <CalendarDays className="size-3" />
            {deadline}
          </div>
        )}
        <Badge variant="outline" className={statusCfg.badgeClass}>
          {statusCfg.label}
        </Badge>
        <ArrowUpRight className="size-3.5 text-muted-foreground opacity-0 transition group-hover:opacity-100" />
      </div>
    </Link>
  );
}

function GoalsListSkeleton() {
  return (
    <div className="rounded-md border bg-card divide-y overflow-hidden">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="flex items-center justify-between gap-4 px-4 py-3"
        >
          <div className="flex items-center gap-3 min-w-0">
            <Skeleton className="size-4 rounded shrink-0" />
            <div className="min-w-0 space-y-1">
              <Skeleton className="h-4 w-48 rounded" />
              <Skeleton className="h-3 w-32 rounded" />
            </div>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <Skeleton className="h-3 w-16 rounded" />
            <Skeleton className="h-5 w-14 rounded" />
            <Skeleton className="size-3.5 rounded" />
          </div>
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

function GoalsSection({ title, subtitle, action, children }: SectionProps) {
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

export function ProjectGoalsTab({ project }: ProjectGoalsTabProps) {
  const navigate = useNavigate();
  const [selectedStatus, setSelectedStatus] =
    React.useState<Goal["status"]>("active");

  const goalQuery = aos.client.goal.list.useQuery({
    query: { project: project.id, limit: "50" },
    staleTime: 5 * 60 * 1000,
  });

  const goals: Goal[] = goalQuery.data?.goals ?? [];
  const isLoading = useDelayedLoading(goalQuery.isLoading);

  const filteredGoals = React.useMemo(
    () => goals.filter((g) => g.status === selectedStatus),
    [goals, selectedStatus],
  );

  return (
    <div className="container mx-auto max-w-3xl py-6 pb-12 space-y-6">
      <GoalsSection
        title={t("Goals")}
        action={
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              void navigate({ to: "/goals/$id", params: { id: "new" } })
            }
          >
            <Plus className="size-4" />
            {t("New Goal")}
          </Button>
        }
      >
        <TabsSubtle
          activeLabel
          selectedIndex={GOAL_STATUS_ORDER.indexOf(selectedStatus)}
          onSelect={(index) => setSelectedStatus(GOAL_STATUS_ORDER[index])}
        >
          {GOAL_TABS.map((tab, index) => (
            <TabsSubtleItem
              key={tab.status}
              index={index}
              label={tab.label}
              icon={tab.icon}
            />
          ))}
        </TabsSubtle>

        <div className="mt-3">
          {isLoading ? (
            <GoalsListSkeleton />
          ) : filteredGoals.length === 0 ? (
            <div className="rounded-md border-2 border-dotted h-12 flex items-center justify-center w-full">
              <span className="text-xs text-muted-foreground/60">
                {t("No goals with this status.")}
              </span>
            </div>
          ) : (
            <div className="rounded-md border bg-card divide-y overflow-hidden">
              {filteredGoals.map((goal) => (
                <GoalRow key={goal.id} goal={goal} />
              ))}
            </div>
          )}
        </div>
      </GoalsSection>
    </div>
  );
}
