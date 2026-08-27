import { aos } from "@/app/aos";
import { stores } from "@/app/lib/stores";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Page, PageBody } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverDescription,
  PopoverTrigger,
} from "@/components/ui/popover";
import { TabsSubtle, TabsSubtleItem } from "@/components/ui/tabs-subtle";
import {
  TASK_STATUS_CONFIG,
  TASK_STATUS_ORDER,
} from "@/features/task/presentation/consts/task";
import {
  GOAL_STATUS_CONFIG,
  GOAL_STATUS_ORDER,
} from "@/features/goal/presentation/consts/goal";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import type { Goal } from "@/features/goal/interfaces/goal.interfaces";
import { ViewStore } from "@/features/view/presentation/stores/view.store";
import { ArtifactStore } from "@/features/artifact/presentation/stores/artifact.store";
import { ArtifactHelper } from "@/features/artifact/presentation/helpers/artifact.helper";
import type { ArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";
import { useNavigate, Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import {
  AppWindow,
  CheckCircle2,
  CirclePlayIcon,
  ChevronDown,
  Flag,
  Folder,
  LayoutGrid,
  PlusSquare,
} from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { InboxPanel } from "../../components/panels/inbox";
import { TaskListRow } from "@/features/task/presentation/pages/(main)/components/list/components/task-list-row.component";
import { GoalListRow } from "@/features/goal/presentation/pages/(main)/components/list/components/goal-list-row.component";
import { WelcomeDialog } from "../../components/dialogs/welcome/welcome-dialog";
import { t } from "@/lib/i18n";

const TASK_TABS = TASK_STATUS_ORDER.map((status) => ({
  status,
  ...TASK_STATUS_CONFIG[status],
}));

const GOAL_TABS = GOAL_STATUS_ORDER.map((status) => ({
  status,
  ...GOAL_STATUS_CONFIG[status],
}));

export const HomePage = aos
  .page("/")
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, context }) => {
    // Route context (AOS's global `withContext(...)`) is unwired in
    // this port — same gap as `aos.useContext()` elsewhere. `DefaultContext`
    // (`app/builders/types.ts`) is deliberately loose (`Record<string,
    // any>`) for exactly this unset case, so no per-call-site cast is
    // needed here.
    const [tasksResult, goalsResult, projectsResult] = await Promise.all([
      client.task.list.query({
        query: {},
      }),
      client.goal.list.query({
        query: {},
      }),
      client.project.list.query({
        query: {},
      }),
    ]);

    const tasks: Task[] = tasksResult.data?.tasks || [];
    const goals: Goal[] = goalsResult.data?.goals || [];
    const projects = projectsResult.data?.projects || [];
    const views = ViewStore.state.items;
    const artifacts = ArtifactStore.state.items;

    return {
      workspace: context.workspaces?.current,
      tasks,
      goals,
      projects,
      views,
      artifacts,
      user: context.config.user,
    };
  })
  .withComponent(({ route }) => {
    const { tasks, goals, projects, views, artifacts, user } = route.useLoaderData();
    const navigate = useNavigate();
    const [selectedStatus, setSelectedStatus] =
      useState<Task["status"]>("suggestion");
    const [selectedGoalStatus, setSelectedGoalStatus] =
      useState<Goal["status"]>("active");
    const [createOpen, setCreateOpen] = useState(false);

    const filteredTasks = useMemo(() => {
      return tasks
        .filter((task) => task.status === selectedStatus)
        .slice(0, 10);
    }, [tasks, selectedStatus]);

    const filteredGoals = useMemo(() => {
      return goals
        .filter((goal) => goal.status === selectedGoalStatus)
        .slice(0, 6);
    }, [goals, selectedGoalStatus]);

    const openView = (viewId: string) => {
      void navigate({ to: "/views/$id", params: { id: viewId } });
    };

    const openArtifact = (artifact: ArtifactListItem) => {
      ArtifactHelper.openInBrowserTab(artifact);
    };

    const openCreate = (kind: "task" | "goal" | "project") => {
      setCreateOpen(false);

      if (kind === "task") {
        stores.viewport.actions.toggle("tasks.dialog.visible", true);
        return;
      }

      if (kind === "goal") {
        void navigate({ to: "/goals/$id", params: { id: "new" } });
        return;
      }

      void navigate({ to: "/projects/$id", params: { id: "new" } });
    };

    return (
      <Page>
        <PageBody className="overflow-hidden p-0">
          <div className="grid h-full min-h-0 grid-cols-[1fr_auto]">
            <div className="mx-auto flex max-w-full w-5xl min-h-0 flex-col gap-6 overflow-y-auto px-6 py-8">
              <section className="space-y-6">
                <div className="flex flex-wrap items-end justify-between gap-4">
                  <div className="space-y-2">
                    <p className="text-sm text-muted-foreground">
                      {new Intl.DateTimeFormat("en-US", {
                        weekday: "long",
                        day: "numeric",
                        month: "short",
                        year: "numeric",
                      }).format(new Date())}
                    </p>

                    <h1 className="text-xl font-semibold tracking-tight">
                      <span className="text-muted-foreground">{t("Hello,")}</span>{" "}
                      {user.name}
                    </h1>
                  </div>
                </div>
                <div className="flex items-center justify-between gap-4">
                  <div className="flex w-fit overflow-hidden rounded-md border bg-card backdrop-blur divide-x">
                    {[
                      {
                        label: "Tasks shipped",
                        value: String(
                          tasks.filter((task) => task.status === "finished")
                            .length,
                        ),
                        icon: CheckCircle2,
                      },
                      {
                        label: "In progress",
                        value: String(
                          tasks.filter((task) => task.status === "in_progress")
                            .length,
                        ),
                        icon: CirclePlayIcon,
                      },
                      {
                        label: "Views",
                        value: String(views.length),
                        icon: LayoutGrid,
                      },
                      {
                        label: "Artifacts",
                        value: String(artifacts.length),
                        icon: AppWindow,
                      },
                      {
                        label: "Goals",
                        value: String(goals.length),
                        icon: Flag,
                      },
                      {
                        label: "Projects",
                        value: String(projects.length),
                        icon: Folder,
                      },
                    ].map((metric) => {
                      const MetricIcon = metric.icon;

                      return (
                        <div
                          key={metric.label}
                          className="flex h-8 items-center gap-3 px-3"
                        >
                          <MetricIcon className="size-4" />
                          <div className="text-sm font-medium">
                            {metric.value}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {metric.label}
                          </div>
                        </div>
                      );
                    })}
                  </div>

                  <Popover open={createOpen} onOpenChange={setCreateOpen}>
                    <PopoverTrigger asChild>
                      <Button
                        size="lg"
                        variant="outline"
                        className="rounded-md"
                      >
                        <PlusSquare />
                        {t("Create")}
                        <ChevronDown className="size-4 text-muted-foreground" />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent
                      align="end"
                      className="w-64 rounded-xl border bg-card/95 p-1 shadow-lg backdrop-blur-xl"
                    >
                      <PopoverHeader className="px-3 py-2">
                        <PopoverTitle>{t("Create")}</PopoverTitle>
                        <PopoverDescription className="text-xs">
                          {t("Start a new item in this workspace.")}
                        </PopoverDescription>
                      </PopoverHeader>
                      <div className="grid gap-1">
                        {[
                          {
                            label: "Task",
                            description: "Capture work, bugs, or next actions.",
                            icon: CheckCircle2,
                            action: "task" as const,
                          },
                          {
                            label: "Goal",
                            description:
                              "Define an outcome to track over time.",
                            icon: Flag,
                            action: "goal" as const,
                          },
                          {
                            label: "Project",
                            description:
                              "Group related work under one initiative.",
                            icon: Folder,
                            action: "project" as const,
                          },
                        ].map((item) => {
                          const ItemIcon = item.icon;

                          return (
                            <button
                              key={item.label}
                              type="button"
                              onClick={() => openCreate(item.action)}
                              className="flex items-start gap-3 rounded-lg px-3 py-2 text-left transition hover:bg-accent/50"
                            >
                              <div className="mt-0.5 rounded-md border bg-background p-1.5 text-muted-foreground">
                                <ItemIcon className="size-4" />
                              </div>
                              <div className="min-w-0 space-y-0.5">
                                <div className="text-sm font-medium">
                                  {item.label}
                                </div>
                                <div className="text-xs text-muted-foreground">
                                  {item.description}
                                </div>
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
              </section>

              <div className="grid gap-6">
                <section>
                  <header className="flex items-center justify-between gap-4 py-4">
                    <h2 className="text-md">{t("Views")}</h2>
                  </header>
                  <main className="flex gap-3">
                    {views.length === 0 && (
                      <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
                        <span className="text-muted-foreground/60">
                          {t("No views on this workspace yet!")}
                        </span>
                      </div>
                    )}

                    {views.slice(0, 12).map((view) => (
                      <button
                        key={view.id}
                        onClick={() => openView(view.name)}
                        className="group w-32 rounded-md border bg-background/70 p-4 text-left transition hover:-translate-y-0.5 hover:border-primary/40 hover:bg-accent/30"
                      >
                        <div className="mb-6 flex items-start justify-between gap-3">
                          <Icon
                            value={view.metadata?.icon as string | undefined}
                            className="size-4"
                          />
                          <ChevronDown className="size-4 -rotate-90 text-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-foreground" />
                        </div>
                        <div className="space-y-1">
                          <div className="font-medium leading-none">
                            {view.title}
                          </div>
                          <div className="line-clamp-2 text-xs text-muted-foreground">
                            {view.description ||
                              "Open the view page and continue working from its dedicated layout."}
                          </div>
                        </div>
                      </button>
                    ))}
                  </main>
                </section>

                <section>
                  <header className="flex items-center justify-between gap-4 py-4">
                    <h2 className="text-md">{t("Artifacts")}</h2>
                  </header>
                  <main className="flex gap-3">
                    {artifacts.length === 0 && (
                      <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
                        <span className="text-muted-foreground/60">
                          {t("No artifacts on this workspace yet!")}
                        </span>
                      </div>
                    )}

                    {artifacts.slice(0, 12).map((artifact) => (
                      <button
                        key={artifact.id}
                        onClick={() => openArtifact(artifact)}
                        className="group w-32 rounded-md border bg-background/70 p-4 text-left transition hover:-translate-y-0.5 hover:border-primary/40 hover:bg-accent/30"
                      >
                        <div className="mb-6 flex items-start justify-between gap-3">
                          <Icon
                            value={ArtifactHelper.getIcon()}
                            fallback="AppWindow"
                            className="size-4"
                          />
                          <ChevronDown className="size-4 -rotate-90 text-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-foreground" />
                        </div>
                        <div className="space-y-1">
                          <div className="font-medium leading-none">
                            {artifact.name}
                          </div>
                          <div className="line-clamp-2 text-xs text-muted-foreground">
                            {artifact.description ||
                              "Open the artifact in a browser tab and continue working from its hosted app."}
                          </div>
                        </div>
                      </button>
                    ))}
                  </main>
                </section>

                <section>
                  <header className="flex flex-col items-start justify-between gap-4 py-4">
                    <div className="flex w-full items-center justify-between gap-4">
                      <h2 className="text-md">{t("Tasks")}</h2>
                      <Link
                        to="/tasks"
                        className="text-sm text-muted-foreground transition hover:text-foreground"
                      >
                        {t("Open all")}
                      </Link>
                    </div>

                    <TabsSubtle
                      activeLabel
                      selectedIndex={TASK_STATUS_ORDER.indexOf(selectedStatus)}
                      onSelect={(index) =>
                        setSelectedStatus(TASK_STATUS_ORDER[index])
                      }
                    >
                      {TASK_TABS.map((status, index) => (
                        <TabsSubtleItem
                          key={status.status}
                          index={index}
                          label={status.label}
                          icon={status.icon}
                        />
                      ))}
                    </TabsSubtle>
                  </header>

                  <main>
                    {filteredTasks.length === 0 ? (
                      <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
                        <span className="text-muted-foreground/60">
                          {t("No tasks in this status.")}
                        </span>
                      </div>
                    ) : (
                      <div className="divide-y rounded-md border bg-card">
                        {filteredTasks.map((task) => {
                          return <TaskListRow key={task.id} task={task} />;
                        })}
                      </div>
                    )}
                  </main>
                </section>

                <section>
                  <header className="flex flex-col items-start justify-between gap-4 py-4">
                    <div className="flex w-full items-center justify-between gap-4">
                      <h2 className="text-md">{t("Goals")}</h2>
                      <Link
                        to="/goals"
                        className="text-sm text-muted-foreground transition hover:text-foreground"
                      >
                        {t("Open all")}
                      </Link>
                    </div>

                    <TabsSubtle
                      activeLabel
                      selectedIndex={GOAL_STATUS_ORDER.indexOf(
                        selectedGoalStatus,
                      )}
                      onSelect={(index) =>
                        setSelectedGoalStatus(GOAL_STATUS_ORDER[index])
                      }
                    >
                      {GOAL_TABS.map((status, index) => (
                        <TabsSubtleItem
                          key={status.status}
                          index={index}
                          label={status.label}
                          icon={status.icon}
                        />
                      ))}
                    </TabsSubtle>
                  </header>

                  <main>
                    {filteredGoals.length === 0 ? (
                      <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
                        <span className="text-muted-foreground/60">
                          {t("No goals in this status.")}
                        </span>
                      </div>
                    ) : (
                      <div className="divide-y rounded-md border bg-card">
                        {filteredGoals.map((goal) => {
                          return <GoalListRow key={goal.id} goal={goal} />;
                        })}
                      </div>
                    )}
                  </main>
                </section>
              </div>
            </div>

            <InboxPanel className="h-full min-h-0" />
          </div>
          <WelcomeDialog />
        </PageBody>
      </Page>
    );
  })
  .build();
