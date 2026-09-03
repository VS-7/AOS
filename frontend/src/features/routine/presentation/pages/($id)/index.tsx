import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import { z } from "zod";
import {
  BotIcon,
  ChevronDown,
  History,
  PlayIcon,
  Save,
  Settings2,
  TagIcon,
  Trash2,
  UserIcon,
} from "lucide-react";

import { Avatar, AvatarAgentFallback } from "@/components/ui/avatar";
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
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
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
import {
  TabsSubtle,
  TabsSubtleItem,
  TabsSubtlePanel,
} from "@/components/ui/tabs-subtle";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import {
  SetRoutineAgentDropdown,
  SetRoutineStatusDropdown,
} from "@/features/routine/presentation/components/dropdowns";
import { RoutineTriggersField } from "@/features/routine/presentation/components/triggers";
import { ROUTINE_STATUS_CONFIG } from "@/features/routine/presentation/consts/routine";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { t } from "@/lib/i18n";
import {
  RoutineTriggerFormSchema,
  RoutineTriggersHelper,
} from "@/features/routine/presentation/helpers/routine-triggers.helper";
import type {
  ActivityEventDefinition,
} from "@/features/activity/interfaces/activity.interfaces";
import type {
  Routine,
  RoutineWithRuns,
} from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineRunHistory,
  RoutineRunHistoryToolbar,
  useRoutineRunHistoryFilters,
} from "./components/routine-run-history";

const routineFormSchema = z.object({
  name: z.string().min(1, "Name is required"),
  prompt: z.string().min(1, "Prompt is required"),
  agent: z.string().min(1, "Agent is required"),
  // Task 9 addition: `RoutineStatusSchema` (routine.interfaces.ts)
  // already had "paused" as a third value — this form schema predates
  // that and only had two.
  status: z.enum(["enabled", "paused", "disabled"]),
  triggers: z.array(RoutineTriggerFormSchema),
});

type RoutineFormValues = z.infer<typeof routineFormSchema>;

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return "Unable to save this routine.";
}

function buildFormValues(routine: Routine | null): RoutineFormValues {
  if (!routine) {
    return {
      name: "",
      prompt: "",
      agent: "",
      status: "enabled",
      triggers: [],
    };
  }

  return {
    name: routine.name,
    prompt: routine.content,
    agent: routine.agent,
    status: routine.status,
    triggers: RoutineTriggersHelper.buildFormTriggers(routine),
  };
}

export const RoutineUpsertPage = aos
  .page("/routines/$id")
  .withMetadata({
    title: "Routine",
    description: "Create and edit agent routines",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const isCreate = request.params.id === "new";

    // No filter: the catalogue *is* the set of events a routine can react to,
    // so `routine: true` named a distinction Go does not make.
    const eventsResult = await client.activity.listEvents.query({});
    const activityEvents = eventsResult.data ?? [];

    if (isCreate) {
      return {
        mode: "create" as const,
        routine: null as RoutineWithRuns | null,
        activityEvents,
      };
    }

    const result = await client.routine.getById.query({
      params: { routine: request.params.id },
    });

    const routine = result.data?.routine;

    if (!routine) {
      return response.notFound();
    }

    return {
      mode: "edit" as const,
      routine,
      activityEvents,
    };
  })
  .withComponent(({ route }) => {
    const router = useRouter();
    const { mode, routine, activityEvents } = route.useLoaderData();
    const routineId = route.useParams().id;
    const isEditMode = mode === "edit";
    const [contentTab, setContentTab] = React.useState(0);
    const tabsIdPrefix = `routine-detail-${routineId}`;
    const runHistoryFilters = useRoutineRunHistoryFilters();

    const agents = aos.stores.agent.useState((state) => state.items);

    const form = aos.useForm({
      schema: routineFormSchema,
      values: buildFormValues(routine),
      onSubmit: async (values: RoutineFormValues) => {
        const body = {
          name: values.name,
          prompt: values.prompt,
          agent: values.agent,
          status: values.status,
          triggers: RoutineTriggersHelper.toApiTriggers(
            values.triggers,
            routine,
          ),
        };

        if (isEditMode && routine) {
          const result = await aos.client.routine.update.mutate({
            params: { routine: routineId },
            body,
          });

          if (result?.error) {
            toast.error(getErrorMessage(result.error));
            return;
          }

          toast.success(t("Routine updated."));
          router.invalidate();
          return;
        }

        const result = await aos.client.routine.create.mutate({ body });

        if (result?.error || !result.data?.routine?.id) {
          toast.error(getErrorMessage(result?.error));
          return;
        }

        toast.success(t("Routine created."));
        router.invalidate();
        router.navigate({
          to: "/routines/$id",
          params: { id: result.data.routine.id },
        });
      },
    });

    const currentAgentId = form.watch("agent");
    const agentLabel = RoutineHelper.getAgentLabel(currentAgentId, agents);

    const { mutate: deleteRoutine, loading: isDeleting } =
      aos.client.routine.delete.useMutation({
        onSuccess: async () => {
          toast.success(t("Routine deleted."));
          await router.invalidate();
          await router.navigate({ to: "/" });
        },
        onError: (error) => {
          toast.error(getErrorMessage(error));
        },
      });

    const routineFireUrl = isEditMode ? routine?.fireUrl : null;

    const { mutate: fireRoutine, loading: isFiring } =
      aos.client.routine.fire.useMutation({
        onSuccess: async (result) => {
          // `onSuccess` receives the full `Envelope` — see `aos-facade.ts`'s
          // `useMutation` doc comment.
          //
          // task-12 (round 2): Go's `routines_fire`
          // (`internal/domain/routine/commands.go`) returns one bare `*Run`
          // — never `{ executions: [...] }`. This UI was written against a
          // backend whose `fire` could fan a routine out to several agents
          // in one call and return the list; this Go port always fires
          // exactly one. `wrapOut` (`command-map.ts`) can only nest a
          // value, not change its cardinality, so this is a call-site
          // adaptation: wrap the single `Run` Go actually returned in a
          // one-element array, so `executionCount` reflects what really
          // happened (one run, or zero if the call returned nothing) —
          // reading `.executions` off a bare `Run` was always `undefined`,
          // silently defaulting to "1" via `?? 1` whether the call
          // succeeded or not.
          const executions = result?.data ? [result.data] : [];
          const executionCount = executions.length;
          toast.success(
            executionCount > 1
              ? `Routine started for ${executionCount} agents.`
              : "Routine started.",
          );
          await router.invalidate();
          setContentTab(1);
        },
        onError: (error) => {
          toast.error(getErrorMessage(error));
        },
      });

    const runs = routine?.runs ?? [];

    async function persistField(
      body: Partial<Pick<RoutineFormValues, "status" | "agent">>,
      successMessage: string,
    ) {
      if (!isEditMode) return;

      const result = await aos.client.routine.update.mutate({
        params: { routine: routineId },
        body,
      });

      if (result?.error) {
        toast.error(getErrorMessage(result.error));
        return;
      }

      toast.success(successMessage);
      await router.invalidate();
    }

    async function handleStatusChange(status: RoutineFormValues["status"]) {
      form.setValue("status", status, { shouldDirty: true });
      await persistField(
        { status },
        `Status updated to ${ROUTINE_STATUS_CONFIG[status].label}`,
      );
    }

    async function handleAgentChange(agent: string) {
      form.setValue("agent", agent, { shouldDirty: true });
      const label = RoutineHelper.getAgentLabel(agent, agents);
      await persistField({ agent }, `Assigned to ${label}`);
    }

    return (
      <Page className="h-full overflow-hidden">
        <PageBody className="overflow-hidden">
          <Form form={form} className="flex h-full flex-1 flex-col">
            <SplitPageLayout>
              <SplitPageLayout.Content>
                <SplitPageLayout.ContentHeader>
                  <SplitPageLayout.ContentHeaderMain className="items-center">
                    <SplitPageLayout.ContentTitle>
                      {isEditMode ? routine?.name : "New Routine"}
                    </SplitPageLayout.ContentTitle>
                  </SplitPageLayout.ContentHeaderMain>

                  <SplitPageLayout.ContentHeaderActions>
                    <TooltipProvider>
                      <div className="flex items-center gap-2">
                        {isEditMode ? (
                          <AlertDialog>
                            <Tooltip>
                              <AlertDialogTrigger asChild>
                                <TooltipTrigger asChild>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    className="rounded-md"
                                    disabled={isDeleting}
                                  >
                                    <Trash2 />
                                    <span className="sr-only">
                                      {t("Delete routine")}
                                    </span>
                                  </Button>
                                </TooltipTrigger>
                              </AlertDialogTrigger>
                              <TooltipContent sideOffset={8}>
                                {t("Delete routine")}
                              </TooltipContent>
                            </Tooltip>
                            <AlertDialogContent size="sm">
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  {t("Delete this routine?")}
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t("This action removes")}{" "}
                                  <strong>{routine?.name}</strong> {t("and all its runs.")}
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
                                    deleteRoutine({ params: { routine: routineId } })
                                  }
                                >
                                  {isDeleting
                                    ? "Deleting..."
                                    : "Delete routine"}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        ) : null}

                        {isEditMode ? (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={isFiring}
                            onClick={() =>
                              fireRoutine({
                                params: { routine: routine!.id },
                                query: {},
                                body: {},
                              })
                            }
                          >
                            <PlayIcon />
                            {isFiring ? "Running..." : "Run now"}
                          </Button>
                        ) : null}

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
                              : "Create routine"}
                        </Button>
                      </div>
                    </TooltipProvider>
                  </SplitPageLayout.ContentHeaderActions>
                </SplitPageLayout.ContentHeader>

                <SplitPageLayout.ContentBody>
                  <div className="container mx-auto flex max-w-3xl flex-col gap-4 py-6 pb-10">
                    <FormField
                      control={form.control}
                      name="name"
                      render={({ field }) => (
                        <FormItem className="space-y-2">
                          <FormLabel className="opacity-60">{t("Name")}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t("Daily Report")}
                              className="h-auto rounded-none border-0 bg-transparent px-0 py-0 text-2xl font-semibold shadow-none focus-visible:ring-0"
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <div className="flex w-full items-center justify-between gap-3">
                      <TabsSubtle
                        activeLabel
                        selectedIndex={contentTab}
                        onSelect={setContentTab}
                        idPrefix={tabsIdPrefix}
                      >
                        <TabsSubtleItem
                          index={0}
                          icon={Settings2}
                          label={t("Settings")}
                        />
                        {isEditMode ? (
                          <TabsSubtleItem
                            index={1}
                            icon={History}
                            label={t("Run History")}
                          />
                        ) : null}
                      </TabsSubtle>

                      {contentTab === 1 && isEditMode ? (
                        <RoutineRunHistoryToolbar filters={runHistoryFilters} />
                      ) : null}
                    </div>

                    <TabsSubtlePanel
                      index={0}
                      selectedIndex={contentTab}
                      idPrefix={tabsIdPrefix}
                      className="space-y-6"
                    >
                      <RoutineTriggersField
                        control={form.control}
                        fireUrl={routineFireUrl}
                        activityEvents={activityEvents}
                      />

                      <FormField
                        control={form.control}
                        name="prompt"
                        render={({ field }) => (
                          <FormItem>
                            <MarkdownEditor
                              value={field.value ?? ""}
                              onValueChange={field.onChange}
                              title={t("Prompt")}
                              placeholder={t("Enter the system prompt for this routine...")}
                            />
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </TabsSubtlePanel>

                    <TabsSubtlePanel
                      index={1}
                      selectedIndex={contentTab}
                      idPrefix={tabsIdPrefix}
                    >
                      {isEditMode ? (
                        <RoutineRunHistory
                          runs={runs}
                          filters={runHistoryFilters}
                          routineName={routine?.name}
                          isFiring={isFiring}
                          onRunNow={() =>
                            fireRoutine({
                              params: { routine: routine!.id },
                              query: {},
                              body: {},
                            })
                          }
                        />
                      ) : null}
                    </TabsSubtlePanel>
                  </div>
                </SplitPageLayout.ContentBody>
              </SplitPageLayout.Content>

              <SplitPageLayout.Detail>
                <SplitPageLayout.DetailTabs defaultValue="overview">
                  <SplitPageLayout.DetailTab
                    value="overview"
                    label={t("Overview")}
                    icon={PlayIcon}
                  >
                    <div className="space-y-3">
                      <SplitPageLayout.Widget>
                        <SplitPageLayout.WidgetHeader>
                          <SplitPageLayout.WidgetTitle>
                            {t("Configuration")}
                          </SplitPageLayout.WidgetTitle>
                        </SplitPageLayout.WidgetHeader>
                        <SplitPageLayout.WidgetContent>
                          <FormField
                            control={form.control}
                            name="status"
                            render={({ field }) => {
                              const config = ROUTINE_STATUS_CONFIG[field.value];
                              const Icon = config.icon;

                              return (
                                <FormItem className="border-0 p-0">
                                  <SplitPageLayout.WidgetItem>
                                    <TagIcon className="size-3.5 shrink-0 text-muted-foreground" />
                                    <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                      {t("Status")}
                                    </span>

                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <button className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent">
                                          <Icon
                                            className={`size-3.5 shrink-0 ${config.color}`}
                                          />
                                          <span>{config.label}</span>
                                          <ChevronDown className="size-3" />
                                        </button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent align="start">
                                        <SetRoutineStatusDropdown
                                          currentStatus={field.value}
                                          onStatusChange={handleStatusChange}
                                        />
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </SplitPageLayout.WidgetItem>
                                </FormItem>
                              );
                            }}
                          />

                          <FormField
                            control={form.control}
                            name="agent"
                            render={({ field }) => (
                              <FormItem className="border-0 p-0">
                                <SplitPageLayout.WidgetItem>
                                  <BotIcon
                                    className={`size-3.5 shrink-0 text-muted-foreground`}
                                  />
                                  <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                    {t("Agent")}
                                  </span>

                                  <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                      <button
                                        type="button"
                                        className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                                      >
                                        {currentAgentId ? (
                                          <Avatar className="size-4">
                                            <AvatarAgentFallback
                                              name={currentAgentId.toLowerCase()}
                                            />
                                          </Avatar>
                                        ) : (
                                          <UserIcon className="size-4" />
                                        )}
                                        <span className="max-w-40 truncate">
                                          {agentLabel || "Select agent"}
                                        </span>
                                        <ChevronDown className="size-3" />
                                      </button>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent
                                      align="start"
                                      className="w-64"
                                    >
                                      <SetRoutineAgentDropdown
                                        currentAgent={field.value}
                                        onAgentChange={handleAgentChange}
                                      />
                                    </DropdownMenuContent>
                                  </DropdownMenu>
                                </SplitPageLayout.WidgetItem>
                              </FormItem>
                            )}
                          />
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
    );
  })
  .build();
