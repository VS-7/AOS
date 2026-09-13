import * as React from "react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import { z } from "zod";
import {
  CalendarDays,
  Check,
  ChevronDown,
  Copy,
  Folder,
  Link2,
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
import { DateTimeInput } from "@/components/ui/date-time-input";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ProjectSelectorDropdown } from "@/components/ui/project-selector-dropdown";
import {
  type GoalPriority,
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
  GOAL_STATUS_ORDER,
} from "@/features/goal/presentation/consts/goal";
import {
  goalCreateBody,
  goalUpdateBody,
  type GoalFormFields,
} from "@/features/goal/presentation/helpers/goal-form";
import { shareableLink } from "@/features/goal/presentation/helpers/goal-link";
import { useStatusTabs } from "@/features/goal/presentation/helpers/status-tabs";
import { hasSluggableCharacter } from "@/features/project/presentation/helpers/project-form";
import { TabsSubtle, TabsSubtleItem } from "@/components/ui/tabs-subtle";
import { TaskListRow } from "@/features/task/presentation/pages/(main)/components/list/components/task-list-row.component";
import {
  TASK_STATUS_CONFIG,
  TASK_STATUS_ORDER,
} from "@/features/task/presentation/consts/task";
import type { Task } from "@/features/task/interfaces/task.interfaces";
import { t, useTranslation } from "@/lib/i18n";

const goalFormSchema = z.object({
  // The messages are resolved when the schema validates, not when this
  // module loads, so they follow the language the person has at that moment.
  // The daemon derives the goal's id from its title, and a title of only
  // symbols derives none and is refused — so that is said here, on the field.
  title: z
    .string()
    .trim()
    .superRefine((value, ctx) => {
      if (!value) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, message: t("Title is required") });
      } else if (!hasSluggableCharacter(value)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("Use at least one letter or number in the title"),
        });
      }
    }),
  description: z.string().optional(),
  measure: z.string().optional(),
  content: z.string().optional(),
  priority: GoalPrioritySchema.default("no_priority"),
  project: z.string().optional(),
  deadline: z.string().optional(),
  status: GoalStatusSchema.default("active"),
});

type GoalFormValues = z.infer<typeof goalFormSchema>;

const GoalDetailsSearchSchema = z.object({
  /** Preselects the project of a goal created from a project's Goals tab. */
  project: z.string().optional(),
});

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return t("Unable to save this goal.");
}

function buildFormValues(
  goal: GoalWithContext | null,
  prefill: { project?: string } = {},
): GoalFormValues {
  if (!goal) {
    return {
      title: "",
      description: "",
      measure: "",
      content: "",
      priority: "no_priority",
      project: prefill.project ?? "",
      deadline: "",
      status: "active",
    };
  }

  return {
    title: goal.title,
    description: goal.description ?? "",
    measure: goal.measure ?? "",
    content: goal.content ?? "",
    priority: goal.priority ?? "no_priority",
    project: goal.project ?? "",
    // Kept as the daemon wrote it. Cutting it to a date here is what made
    // every save send it back rewritten to midnight UTC.
    deadline: goal.deadline ?? "",
    status: goal.status ?? "active",
  };
}

async function copyToClipboard(value: string, message: string) {
  await navigator.clipboard.writeText(value);
  toast.success(message);
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

/**
 * The tasks that serve this goal, by status.
 *
 * It used to render only when the goal had tasks, open on "Todo" whatever
 * they were, and show a bare icon row — and it never had any, because the
 * loader asked for the literal goal id '["id"]'. It is always there now, opens
 * on the first status with tasks, counts each, and says when there are none.
 */
function GoalTasksSection({ tasks }: { tasks: Task[] }) {
  // Subscribing re-renders the tab labels when the language changes.
  const { t } = useTranslation();
  const { selected, select, counts } = useStatusTabs(TASK_STATUS_ORDER, tasks);
  const visible = React.useMemo(
    () => tasks.filter((task) => task.status === selected).slice(0, 15),
    [tasks, selected],
  );

  return (
    <section className="border-t pt-6">
      <header className="flex items-center gap-1 pb-4">
        <h3 className="text-sm font-medium">{t("Tasks")}</h3>
        <span className="text-sm text-muted-foreground">
          {t("{{count}} total", { count: tasks.length })}
        </span>
      </header>

      {tasks.length === 0 ? (
        <div className="flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
          <span className="text-xs text-muted-foreground/60">
            {t("No tasks serve this goal yet.")}
          </span>
        </div>
      ) : (
        <>
          <TabsSubtle
            activeLabel
            selectedIndex={TASK_STATUS_ORDER.indexOf(selected)}
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

          {visible.length === 0 ? (
            <div className="mt-3 flex h-12 w-full items-center justify-center rounded-md border-2 border-dotted">
              <span className="text-xs text-muted-foreground/60">
                {t("No tasks in this status.")}
              </span>
            </div>
          ) : (
            <div className="mt-3 divide-y rounded-md border bg-card">
              {visible.map((task) => (
                <TaskListRow key={task.id} task={task} />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}

export const GoalDetailsPage = aos
  .page("/goals/$id")
  .withMetadata({
    title: "Goal",
    description: "Create and edit goals",
  })
  .withQuery(GoalDetailsSearchSchema)
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
      // The goal's id, as is. It was sent JSON-encoded ('["id"]'), which
      // tasks_list compares verbatim, so the goal page never listed a task.
      client.task.list.query({
        query: { goal: request.params.id },
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
    const navigate = useNavigate();
    const router = useRouter();
    const { t } = useTranslation();
    const { mode, goal, goalTasks } = route.useLoaderData();
    const search = route.useSearch();
    const goalId = route.useParams().id;
    const isEditMode = mode === "edit";
    const projects = aos.stores.projects.useState((state) => state.items);

    React.useEffect(() => {
      aos.stores.viewport.actions.toggle("page.details.visible", true);
    }, []);

    // react-hook-form applies `values` only on mount: moving from one goal to
    // another on this same route kept the first goal's fields on screen.
    const resetForm = React.useRef<() => void>(() => {});
    React.useEffect(() => {
      resetForm.current();
    }, [goalId]);

    const form = aos.useForm({
      schema: goalFormSchema,
      values: buildFormValues(goal, { project: search.project }),
      onSubmit: async (values: GoalFormValues) => {
        if (isEditMode && goal) {
          const result = await aos.client.goal.update.mutate({
            params: { goal: goalId },
            body: goalUpdateBody(
              values as GoalFormFields,
              buildFormValues(goal) as GoalFormFields,
            ),
          });

          if (result?.error) {
            toast.error(t("Could not save the goal"), {
              description: getErrorMessage(result.error),
            });
            return;
          }

          toast.success(t("Goal updated."));
          void aos.stores.goals.actions.refresh();
          await router.invalidate();
          return;
        }

        const result = await aos.client.goal.create.mutate({
          body: goalCreateBody(values as GoalFormFields),
        });

        const createdGoalId = result?.data?.goal?.id;

        if (result?.error || !createdGoalId) {
          const code = (result?.error as { code?: string } | undefined)?.code;
          // A title the daemon cannot take is the title's problem: say it on
          // the field, where the person is going to fix it.
          if (code === "AOS_GOAL_ALREADY_EXISTS") {
            form.setError("title", {
              message: t("A goal with this title already exists. Choose another title."),
            });
            return;
          }
          if (code === "AOS_GOAL_TITLE_REQUIRED") {
            form.setError("title", {
              message: t("Use at least one letter or number in the title"),
            });
            return;
          }
          toast.error(t("Could not create the goal"), {
            description: getErrorMessage(result?.error),
          });
          return;
        }

        toast.success(t("Goal created."));
        void aos.stores.goals.actions.refresh();
        await navigate({ to: "/goals/$id", params: { id: createdGoalId } });
      },
    });

    resetForm.current = () => form.reset(buildFormValues(goal, { project: search.project }));

    const { mutate: deleteGoal, loading: isDeleting } =
      aos.client.goal.delete.useMutation({
        onSuccess: async () => {
          toast.success(t("Goal deleted."));
          // Leave first. Invalidating while still on /goals/$id reran this
          // page's loader for the goal just removed, which answered
          // GOAL_NOT_FOUND before the navigation happened.
          await navigate({ to: "/goals", replace: true });
          void aos.stores.goals.actions.refresh();
        },
        onError: (error) => {
          toast.error(t("Could not delete the goal"), {
            description: getErrorMessage(error),
          });
        },
      });

    const statusValue = form.watch("status");
    const status =
      GOAL_STATUS_CONFIG[statusValue] ?? GOAL_STATUS_CONFIG.active;
    const StatusIcon = status.icon;
    const currentProject = (projects ?? []).find(
      (project) => project.id === form.watch("project"),
    );

    const link = isEditMode && goal ? shareableLink(`/goals/${goal.id}`) : null;

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
                      {isEditMode ? goal?.title : t("New Goal")}
                    </SplitPageLayout.ContentTitle>
                  </SplitPageLayout.ContentHeaderMain>

                  <SplitPageLayout.ContentHeaderActions>
                    <TooltipProvider>
                      <div className="flex items-center gap-2">
                        {link ? (
                          <HeaderIconButton
                            label={link.label}
                            onClick={() =>
                              void copyToClipboard(link.value, link.copied)
                            }
                          >
                            <Link2 />
                          </HeaderIconButton>
                        ) : null}

                        {isEditMode && goal ? (
                          <HeaderIconButton
                            label={t("Copy ID")}
                            onClick={() =>
                              void copyToClipboard(goal.id, t("Goal ID copied"))
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
                                  {t("This permanently removes {{name}}. Tasks that serve it are kept, without the goal.", {
                                    name: goal.title,
                                  })}
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
                                  {isDeleting ? t("Deleting...") : t("Delete goal")}
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
                          ? t("Saving...")
                          : isEditMode
                            ? t("Save changes")
                            : t("Create goal")}
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

                    {/* The criterion that makes a goal checkable rather than
                        aspirational — the daemon's own guidance asks for one,
                        and no screen showed or edited it. */}
                    <FormField
                      control={form.control}
                      name="measure"
                      render={({ field }) => (
                        <FormItem className="space-y-2">
                          <FormLabel className="opacity-60">
                            {t("Measure")}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              placeholder={t("How will you know this goal was achieved?")}
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

                    {isEditMode ? <GoalTasksSection tasks={goalTasks} /> : null}
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
                                        {GOAL_STATUS_ORDER.map((value) => {
                                          const config = GOAL_STATUS_CONFIG[value];
                                          const Icon = config.icon;

                                          return (
                                            <DropdownMenuItem
                                              key={value}
                                              onClick={() => field.onChange(value)}
                                              className="flex items-center gap-2"
                                            >
                                              <Icon
                                                className={`size-4 ${config.color}`}
                                              />
                                              <span>{config.label}</span>
                                              {field.value === value ? (
                                                <Check className="ml-auto size-3.5" />
                                              ) : null}
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
                                              {field.value === value ? (
                                                <Check className="ml-auto size-3.5" />
                                              ) : null}
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
                                        clearLabel={t("Remove deadline")}
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
                                              t("No project")}
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
