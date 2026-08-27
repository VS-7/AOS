import * as React from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { AnimatePresence, motion } from "framer-motion";
import { toast } from "sonner";
import { z } from "zod";
import {
  CalendarDays,
  ChevronDown,
  Copy,
  Folder,
  Link2,
  ListChecks,
  Save,
  Target,
  Trash2,
} from "lucide-react";

import { aos } from "@/app/aos";
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { Page, PageBody } from "@/components/ui/page";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { Textarea } from "@/components/ui/textarea";
import { Icon } from "@/components/ui/icon";
import { DateTimeInput } from "@/components/ui/date-time-input";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import { cn } from "@/lib/utils";
import {
  type GoalPriority,
  type GoalStatus,
  type GoalWithContext,
} from "@/features/goal/interfaces/goal.interfaces";
import {
  GoalPrioritySchema,
  GoalStatusSchema,
} from "@/features/goal/schemas/goal.schema";
import {
  GOAL_PRIORITY_CONFIG,
  GOAL_PRIORITY_ORDER,
  GOAL_STATUS_CONFIG,
} from "@/features/goal/presentation/consts/goal";
import { GoalHelper } from "@/features/goal/presentation/helpers/goal.helper";
import { TabsSubtle, TabsSubtleItem } from "@/components/ui/tabs-subtle";
import { TaskListRow } from "@/features/task/presentation/pages/(main)/components/list/components/task-list-row.component";
import {
  TASK_STATUS_CONFIG,
  TASK_STATUS_ORDER,
} from "@/features/task/presentation/consts/task";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { t } from "@/lib/i18n";

const goalFormSchema = z.object({
  title: z.string().trim().min(1, "Title is required"),
  description: z.string().optional(),
  content: z.string().optional(),
  priority: GoalPrioritySchema.default("no_priority"),
  project: z.string().optional(),
  deadline: z.string().optional(),
  status: GoalStatusSchema.default("active"),
});

type GoalFormValues = z.infer<typeof goalFormSchema>;

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return "Unable to save this goal.";
}

function toDateInputValue(value?: string) {
  if (!value) return "";
  return value.slice(0, 10);
}

function toDeadlineValue(value?: string) {
  if (!value) return undefined;

  const trimmedValue = value.trim();
  if (!trimmedValue) return undefined;

  return new Date(`${trimmedValue}T00:00:00.000Z`).toISOString();
}

function buildFormValues(goal: GoalWithContext | null): GoalFormValues {
  if (!goal) {
    return {
      title: "",
      description: "",
      content: "",
      priority: "no_priority",
      project: "",
      deadline: "",
      status: "active",
    };
  }

  return {
    title: goal.title,
    description: goal.description ?? "",
    content: goal.content ?? "",
    priority: goal.priority ?? "no_priority",
    project: goal.project ?? "",
    deadline: toDateInputValue(goal.deadline),
    status: goal.status ?? "active",
  };
}

/** Igniter browser client stringifies query arrays with `String(arr)`; Schema.object expects JSON. */
function toJsonArrayQueryParam(values: string[]) {
  return JSON.stringify(values);
}

async function copyToClipboard(value: string, label: string) {
  await navigator.clipboard.writeText(value);
  toast.success(`${label} copied`);
}

interface HeaderIconButtonProps {
  children: React.ReactNode;
  label: string;
  onClick?: () => void | Promise<void>;
}

function HeaderIconButton({ children, label, onClick }: HeaderIconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="rounded-md"
          onClick={onClick}
        >
          {children}
          <span className="sr-only">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent sideOffset={8}>{label}</TooltipContent>
    </Tooltip>
  );
}

export const GoalDetailsPage = aos
  .page("/goals/$id")
  .withMetadata({
    title: "Goal",
    description: "Create and edit goals",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const isCreate = request.params.id === "new";

    // Task 10: the `goal` domain is dormant — no Go backend to call yet.
    // Short-circuits before any client call, the same shape `isCreate`
    // already returns, so `withComponent` below can rely on its existing
    // null-`goal` handling. `DormantGate` (wrapping the returned JSX)
    // is what actually hides the form; this only keeps the loader from
    // calling `response.notFound()` on the dormant command's empty
    // envelope and preempting that render with the 404 page instead.
    if (isDormant("goal") || isCreate) {
      return {
        mode: "create" as const,
        goal: null as GoalWithContext | null,
        goalTasks: [] as Task[],
      };
    }

    const [goalResult, tasksResult] = await Promise.all([
      client.goal.getById.query({
        params: { goal: request.params.id },
      }),
      client.task.list.query({
        query: {
          goal: toJsonArrayQueryParam([request.params.id]) as unknown as string[],
        },
      }),
    ]);
    const goal = goalResult.data?.goal;
    const goalTasks = (tasksResult.data?.tasks ?? []) as Task[];

    if (!goal) {
      return response.notFound();
    }

    return {
      mode: "edit" as const,
      goal,
      goalTasks,
    };
  })
  .withComponent(({ route }) => {
    const router = useRouter();
    const navigate = useNavigate();
    const { mode, goal, goalTasks } = route.useLoaderData();
    const goalId = route.useParams().id;
    const isEditMode = mode === "edit";
    const projects = aos.stores.projects.useState((state) => state.items);

    const [selectedTaskStatus, setSelectedTaskStatus] =
      React.useState<Task["status"]>("todo");

    const filteredTasks = React.useMemo(() => {
      return goalTasks
        .filter((task) => task.status === selectedTaskStatus)
        .slice(0, 15);
    }, [goalTasks, selectedTaskStatus]);

    const TASK_TABS = React.useMemo(
      () =>
        TASK_STATUS_ORDER.map((status) => ({
          status,
          ...TASK_STATUS_CONFIG[status],
        })),
      [],
    );

    React.useEffect(() => {
      aos.stores.viewport.actions.toggle("page.details.visible", true);
    }, []);

    const form = aos.useForm({
      schema: goalFormSchema,
      values: buildFormValues(goal),
      onSubmit: async (values: GoalFormValues) => {
        const body = {
          title: values.title.trim(),
          description: values.description?.trim() || undefined,
          content: values.content?.trim() || undefined,
          priority: values.priority,
          project: values.project?.trim() || undefined,
          deadline: toDeadlineValue(values.deadline),
          status: values.status,
        };

        if (isEditMode && goal) {
          const result = await aos.client.goal.update.mutate({
            params: { goal: goalId },
            body,
          });

          if (result?.error) {
            toast.error(getErrorMessage(result.error));
            return;
          }

          toast.success(t("Goal updated."));
          void aos.stores.goals.actions.refresh();
          await router.invalidate();
          return;
        }

        const result = await aos.client.goal.create.mutate({ body });

        const createdGoalId = result?.data?.goal?.id;

        if (result?.error || !createdGoalId) {
          toast.error(getErrorMessage(result?.error));
          return;
        }

        toast.success(t("Goal created."));
        void aos.stores.goals.actions.refresh();
        await router.invalidate();
        await navigate({ to: "/goals/$id", params: { id: createdGoalId } });
      },
    });

    const { mutate: deleteGoal, loading: isDeleting } =
      aos.client.goal.delete.useMutation({
        onSuccess: async () => {
          toast.success(t("Goal deleted."));
          void aos.stores.goals.actions.refresh();
          await router.invalidate();
          await navigate({ to: "/goals" });
        },
        onError: (error) => {
          toast.error(getErrorMessage(error));
        },
      });

    const statusValue = form.watch("status");
    const priorityValue = form.watch("priority");
    const deadlineValue = form.watch("deadline");

    const status =
      GOAL_STATUS_CONFIG[statusValue] ?? GOAL_STATUS_CONFIG.active;
    const StatusIcon = status.icon;
    const currentProject = (projects ?? []).find(
      (project) => project.id === form.watch("project"),
    );

    const deadlineFormatted = GoalHelper.formatDeadline(
      toDeadlineValue(deadlineValue),
    );
    const isOverdue = GoalHelper.isOverdue(toDeadlineValue(deadlineValue));
    const goalLink =
      isEditMode && goal
        ? typeof window === "undefined"
          ? `/goals/${goal.id}`
          : new URL(`/goals/${goal.id}`, window.location.origin).toString()
        : null;

    return (
      <DormantGate feature="goal">
      <Page className="h-full overflow-hidden">
        <PageBody className="overflow-hidden">
          <Form form={form} className="flex h-full flex-1 flex-col">
            <SplitPageLayout>
              <SplitPageLayout.Content>
                <SplitPageLayout.ContentHeader>
                  <SplitPageLayout.ContentHeaderMain className="items-center gap-3">
                    <Badge variant="outline" className={status.badgeClass}>
                      <StatusIcon className={`size-3 ${status.color}`} />
                      {status.label}
                    </Badge>
                    <SplitPageLayout.ContentTitle>
                      {isEditMode ? goal?.title : "New Goal"}
                    </SplitPageLayout.ContentTitle>
                  </SplitPageLayout.ContentHeaderMain>

                  <SplitPageLayout.ContentHeaderActions>
                    <TooltipProvider>
                      <div className="flex items-center gap-2">
                        {isEditMode && goalLink ? (
                          <HeaderIconButton
                            label={t("Copy link")}
                            onClick={() =>
                              void copyToClipboard(goalLink, "Goal link")
                            }
                          >
                            <Link2 />
                          </HeaderIconButton>
                        ) : null}

                        {isEditMode && goal ? (
                          <HeaderIconButton
                            label={t("Copy ID")}
                            onClick={() =>
                              void copyToClipboard(goal.id, "Goal ID")
                            }
                          >
                            <Copy />
                          </HeaderIconButton>
                        ) : null}

                        {isEditMode && goal ? (
                          <AlertDialog>
                            <Tooltip>
                              <AlertDialogTrigger asChild>
                                <TooltipTrigger asChild>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    className="rounded-md"
                                  >
                                    <Trash2 />
                                    <span className="sr-only">{t("Delete goal")}</span>
                                  </Button>
                                </TooltipTrigger>
                              </AlertDialogTrigger>
                              <TooltipContent sideOffset={8}>
                                {t("Delete goal")}
                              </TooltipContent>
                            </Tooltip>
                            <AlertDialogContent size="sm">
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  {t("Delete this goal?")}
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t("This action removes")}{" "}
                                  <strong>{goal.title}</strong> permanently.
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel disabled={isDeleting}>
                                  {t("Cancel")}
                                </AlertDialogCancel>
                                <AlertDialogAction
                                  variant="destructive"
                                  disabled={isDeleting}
                                  onClick={() =>
                                    deleteGoal({ params: { goal: goal.id } })
                                  }
                                >
                                  {isDeleting ? "Deleting..." : "Delete goal"}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        ) : null}
                      </div>
                    </TooltipProvider>

                    <div className="flex items-center gap-2">
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => void form.submit()}
                        disabled={form.isLoading}
                      >
                        <Save />
                        {form.isLoading
                          ? "Saving..."
                          : isEditMode
                            ? "Save changes"
                            : "Create goal"}
                      </Button>
                    </div>
                  </SplitPageLayout.ContentHeaderActions>
                </SplitPageLayout.ContentHeader>

                <SplitPageLayout.ContentBody>
                  <div className="container mx-auto max-w-3xl space-y-6 py-6 pb-10">
                    <FormField
                      control={form.control}
                      name="title"
                      render={({ field }) => (
                        <FormItem className="space-y-2">
                          <FormLabel className="opacity-60">{t("Title")}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t("Launch V1")}
                              className="h-auto border-0 bg-transparent px-0 py-0 rounded-none text-2xl font-semibold shadow-none focus-visible:ring-0"
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="description"
                      render={({ field }) => (
                        <FormItem className="space-y-2">
                          <FormLabel className="opacity-60">
                            {t("Description")}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              placeholder={t("Briefly describe the outcome this goal is driving.")}
                              className="min-h-10 max-h-48 resize-none border-0 rounded-none bg-transparent px-0 py-0 text-sm shadow-none focus-visible:ring-0 overflow-y-auto"
                              {...field}
                              value={field.value ?? ""}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="content"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className="opacity-60">{t("Content")}</FormLabel>
                          <FormControl>
                            <MarkdownEditor
                              value={field.value ?? ""}
                              onValueChange={field.onChange}
                              placeholder={t("Add the detailed context, milestones, or notes for this goal...")}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    {isEditMode && goalTasks.length > 0 ? (
                      <section className="border-t pt-6">
                        <header className="flex items-center gap-1 pb-4">
                          <h3 className="text-sm font-medium">{t("Tasks")}</h3>
                          <span className="text-sm text-muted-foreground">
                            {goalTasks.length} total
                          </span>
                        </header>

                        <TabsSubtle
                          activeLabel
                          selectedIndex={TASK_STATUS_ORDER.indexOf(
                            selectedTaskStatus,
                          )}
                          onSelect={(index) =>
                            setSelectedTaskStatus(TASK_STATUS_ORDER[index])
                          }
                        >
                          {TASK_TABS.map((tab, index) => (
                            <TabsSubtleItem
                              key={tab.status}
                              index={index}
                              label={tab.label}
                              icon={tab.icon}
                            />
                          ))}
                        </TabsSubtle>

                        {<div className="mt-3 divide-y rounded-md border bg-card">
                            {filteredTasks.map((task) => (
                              <TaskListRow key={task.id} task={task} />
                            ))}
                          </div>}
                      </section>
                    ) : null}
                  </div>
                </SplitPageLayout.ContentBody>
              </SplitPageLayout.Content>

              <SplitPageLayout.Detail>
                <SplitPageLayout.DetailTabs defaultValue="overview">
                  <SplitPageLayout.DetailTab
                    value="overview"
                    label={t("Overview")}
                    icon={Target}
                  >
                    <div className="space-y-3">
                      <SplitPageLayout.Widget>
                        <SplitPageLayout.WidgetHeader>
                          <SplitPageLayout.WidgetTitle>
                            {t("Properties")}
                          </SplitPageLayout.WidgetTitle>
                        </SplitPageLayout.WidgetHeader>
                        <SplitPageLayout.WidgetContent>
                          <FormField
                            control={form.control}
                            name="status"
                            render={({ field }) => {
                              const currentStatus =
                                GOAL_STATUS_CONFIG[field.value] ??
                                GOAL_STATUS_CONFIG.active;
                              const CurrentStatusIcon = currentStatus.icon;

                              return (
                                <FormItem className="border-0 p-0">
                                  <SplitPageLayout.WidgetItem>
                                    <CurrentStatusIcon
                                      className={`size-3.5 shrink-0 ${currentStatus.color}`}
                                    />
                                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                      {t("Status")}
                                    </span>

                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <button
                                          type="button"
                                          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                                        >
                                          <span>{currentStatus.label}</span>
                                          <ChevronDown className="size-3" />
                                        </button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent align="start">
                                        {Object.entries(GOAL_STATUS_CONFIG).map(
                                          ([value, config]) => {
                                            const Icon = config.icon;

                                            return (
                                              <DropdownMenuItem
                                                key={value}
                                                onClick={() =>
                                                  field.onChange(
                                                    value as GoalStatus,
                                                  )
                                                }
                                                className="flex items-center gap-2"
                                              >
                                                <Icon
                                                  className={`size-4 ${config.color}`}
                                                />
                                                <span>{config.label}</span>
                                              </DropdownMenuItem>
                                            );
                                          },
                                        )}
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </SplitPageLayout.WidgetItem>
                                </FormItem>
                              );
                            }}
                          />

                          <FormField
                            control={form.control}
                            name="priority"
                            render={({ field }) => {
                              const currentPriority =
                                GOAL_PRIORITY_CONFIG[field.value] ??
                                GOAL_PRIORITY_CONFIG.no_priority;
                              const CurrentPriorityIcon = currentPriority.icon;

                              return (
                                <FormItem className="border-0 p-0">
                                  <SplitPageLayout.WidgetItem>
                                    <CurrentPriorityIcon
                                      className={`size-3.5 shrink-0 ${currentPriority.colorClass}`}
                                    />
                                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                      {t("Priority")}
                                    </span>

                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <button
                                          type="button"
                                          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                                        >
                                          <span>{currentPriority.label}</span>
                                          <ChevronDown className="size-3" />
                                        </button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent align="start">
                                        {GOAL_PRIORITY_ORDER.map((value) => {
                                          const config =
                                            GOAL_PRIORITY_CONFIG[value];
                                          const Icon = config.icon;

                                          return (
                                            <DropdownMenuItem
                                              key={value}
                                              onClick={() =>
                                                field.onChange(
                                                  value as GoalPriority,
                                                )
                                              }
                                              className="flex items-center gap-2"
                                            >
                                              <Icon
                                                className={`size-4 ${config.colorClass}`}
                                              />
                                              <span>{config.label}</span>
                                            </DropdownMenuItem>
                                          );
                                        })}
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </SplitPageLayout.WidgetItem>
                                </FormItem>
                              );
                            }}
                          />

                          <FormField
                            control={form.control}
                            name="deadline"
                            render={({ field }) => (
                              <FormItem className="border-0 p-0">
                                <SplitPageLayout.WidgetItem className="items-start">
                                  <CalendarDays className="mt-1 size-3.5 shrink-0 text-muted-foreground" />
                                  <span className="w-16 shrink-0 pt-0.5 text-xs text-muted-foreground">
                                    {t("Deadline")}
                                  </span>
                                  <div className="flex min-w-0 flex-1 flex-col gap-2">
                                    <FormControl>
                                      <DateTimeInput
                                        value={field.value}
                                        onValueChange={(value) =>
                                          field.onChange(value ?? "")
                                        }
                                        variant="ghost"
                                        className="h-8"
                                        size="sm"
                                      />
                                    </FormControl>
                                  </div>
                                </SplitPageLayout.WidgetItem>
                                <FormMessage />
                              </FormItem>
                            )}
                          />

                          <FormField
                            control={form.control}
                            name="project"
                            render={({ field }) => (
                              <FormItem className="border-0 p-0">
                                <SplitPageLayout.WidgetItem className="items-start">
                                  <Folder className="mt-1 size-3.5 shrink-0 text-muted-foreground" />
                                  <span className="w-16 shrink-0 pt-0.5 text-xs text-muted-foreground">
                                    {t("Project")}
                                  </span>
                                  <div className="min-w-0 flex-1">
                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <button
                                          type="button"
                                          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                                        >
                                          <span className="truncate">
                                            {currentProject?.name ||
                                              "No project"}
                                          </span>
                                          <ChevronDown className="size-3 shrink-0 text-muted-foreground" />
                                        </button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent
                                        align="start"
                                        className="w-64"
                                      >
                                        <ProjectSelectorDropdown
                                          currentProject={field.value}
                                          onProjectChange={(project) =>
                                            field.onChange(project ?? "")
                                          }
                                        />
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </div>
                                </SplitPageLayout.WidgetItem>
                                <FormMessage />
                              </FormItem>
                            )}
                          />

                          {isEditMode && goal ? (
                            <SplitPageLayout.WidgetItem>
                              <Target className="size-3.5 shrink-0 text-muted-foreground" />
                              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                ID
                              </span>
                              <span className="font-mono text-xs text-muted-foreground pl-1">
                                {goal.id}
                              </span>
                            </SplitPageLayout.WidgetItem>
                          ) : null}
                        </SplitPageLayout.WidgetContent>
                      </SplitPageLayout.Widget>
                    </div>
                  </SplitPageLayout.DetailTab>
                </SplitPageLayout.DetailTabs>
              </SplitPageLayout.Detail>
            </SplitPageLayout>
          </Form>
        </PageBody>
      </Page>
      </DormantGate>
    );
  })
  .build();
