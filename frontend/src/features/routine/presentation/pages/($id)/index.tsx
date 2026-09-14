import * as React from "react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  BotIcon,
  ChevronDown,
  History,
  PlayIcon,
  Save,
  Settings2,
  ShieldIcon,
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
import { useAlert } from "@/components/ui/alert-provider";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Page, PageBody } from "@/components/ui/page";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import {
  TabsSubtle,
  TabsSubtleItem,
  TabsSubtlePanel,
} from "@/components/ui/tabs-subtle";
import { Textarea } from "@/components/ui/textarea";
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
import {
  RoutineTriggersField,
  WebhookTokenDialog,
} from "@/features/routine/presentation/components/triggers";
import { ROUTINE_STATUS_CONFIG } from "@/features/routine/presentation/consts/routine";
import { RoutineHelper } from "@/features/routine/presentation/helpers/routine.helper";
import { describeFireFailure } from "@/features/routine/presentation/helpers/routine-fire.helper";
import {
  RoutineWebhookHelper,
  pendingWebhookTokens,
} from "@/features/routine/presentation/helpers/routine-webhook.helper";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import { RoutineTriggersHelper } from "@/features/routine/presentation/helpers/routine-triggers.helper";
import {
  buildFormValues,
  buildUpdateBody,
  routineFormSchema,
  type RoutineFormValues,
} from "@/features/routine/presentation/helpers/routine-form.helper";
import type {
  Routine,
  RoutineScope,
  RoutineStatus,
  Run,
} from "@/features/routine/interfaces/routine.interfaces";
import {
  RoutineRunHistory,
  RoutineRunHistoryToolbar,
  useRoutineRunHistoryFilters,
} from "./components/routine-run-history";

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
        routine: null as Routine | null,
        runs: [] as Run[],
        activityEvents,
      };
    }

    // The run history is its own command: `routines_get` answers the routine
    // alone, and the page read `routine.runs`, which Go has never carried —
    // so the history said "No runs yet" beside every run on disk.
    const [result, runsResult] = await Promise.all([
      client.routine.getById.query({ params: { routine: request.params.id } }),
      client.routine.runs.query({
        params: { routine: request.params.id },
        query: { limit: 200 },
      }),
    ]);

    const routine = result.data?.routine as Routine | undefined;

    if (!routine) {
      return response.notFound();
    }

    return {
      mode: "edit" as const,
      routine,
      runs: (runsResult.data?.runs ?? []) as Run[],
      activityEvents,
    };
  })
  .withComponent(({ route }) => {
    const router = useRouter();
    const { confirm } = useAlert();
    const { mode, routine, runs, activityEvents } = route.useLoaderData();
    const routineId = route.useParams().id;
    const isEditMode = mode === "edit";
    const [contentTab, setContentTab] = React.useState(0);
    const tabsIdPrefix = `routine-detail-${routineId}`;
    const runHistoryFilters = useRoutineRunHistoryFilters();
    const [shownToken, setShownToken] = React.useState<string | null>(null);
    const [rotating, setRotating] = React.useState(false);

    const agents = aos.stores.agent.useState((state) => state.items);

    const form = aos.useForm({
      schema: routineFormSchema,
      values: buildFormValues(routine),
      onSubmit: async (values: RoutineFormValues) => {
        if (isEditMode && routine) {
          const result = await aos.client.routine.update.mutate({
            params: { routine: routineId },
            body: buildUpdateBody(values, routine),
          });

          if (result?.error) {
            toast.error(t("Failed to save routine"), {
              description: errorMessage(result.error),
            });
            // Thrown so the form stays as edited rather than being reset as
            // though the daemon had taken it.
            throw result.error;
          }

          toast.success(t("Routine updated."));
          // Only a webhook added in this save is handed a token.
          if (result.data?.token) setShownToken(result.data.token);
          await router.invalidate();
          return;
        }

        const result = await aos.client.routine.create.mutate({
          body: {
            name: values.name,
            prompt: values.prompt,
            agent: values.agent,
            status: values.status,
            triggers: RoutineTriggersHelper.toApiTriggers(values.triggers),
            scope: values.scope,
          },
        });

        if (result?.error || !result.data?.routine?.id) {
          toast.error(t("Failed to create routine"), {
            description: errorMessage(result?.error),
          });
          throw result?.error ?? new Error("the routine was not created");
        }

        toast.success(t("Routine created."));
        if (result.data.token) {
          pendingWebhookTokens.hold(result.data.routine.id, result.data.token);
        }
        await router.navigate({
          to: "/routines/$id",
          params: { id: result.data.routine.id },
        });
      },
    });

    // The token a create handed out before navigating here.
    React.useEffect(() => {
      if (!routine?.id) return;
      const token = pendingWebhookTokens.take(routine.id);
      if (token) setShownToken(token);
    }, [routine?.id]);

    // A new routine starts with the workspace's orchestrator rather than with
    // no owner and a Create that silently does nothing.
    const defaultAgentId = RoutineHelper.getDefaultAgentId(agents);
    React.useEffect(() => {
      if (isEditMode || !defaultAgentId || form.getValues("agent")) return;
      form.setValue("agent", defaultAgentId);
    }, [defaultAgentId, form, isEditMode]);

    const currentAgentId = form.watch("agent");
    const agentLabel = RoutineHelper.getAgentLabel(currentAgentId, agents);

    const { mutate: deleteRoutine, loading: isDeleting } =
      aos.client.routine.delete.useMutation({
        onSuccess: async () => {
          toast.success(t("Routine deleted."));
          // Away from the deleted routine first. Invalidating while still on
          // its page re-ran this loader for a routine that no longer exists.
          await router.navigate({ to: "/routines" });
        },
        onError: (error) => {
          toast.error(t("Failed to delete routine"), {
            description: errorMessage(error),
          });
        },
      });

    const { mutate: fireRoutine, loading: isFiring } =
      aos.client.routine.fire.useMutation({
        onSuccess: async () => {
          // `routines_fire` answers once the run has ended, with that run.
          toast.success(t("The routine ran."));
          await router.invalidate();
          setContentTab(1);
        },
        onError: async (error) => {
          // A failed run is still a run: refresh so the history shows it.
          const failure = await describeFireFailure(routineId, error);
          toast.error(failure.title, { description: failure.description });
          await router.invalidate();
          setContentTab(1);
        },
      });

    // Go fires only an enabled routine unless it is told to force it, and
    // refuses otherwise. Offering the same "Run now" on a disabled routine
    // promised a run that could only be refused, so the button says what it
    // will do instead, and does it.
    const runsDespiteStatus = isEditMode && routine?.status !== "enabled";
    const fireNow = () =>
      fireRoutine({
        params: { routine: routineId },
        query: {},
        body: runsDespiteStatus ? { force: true } : {},
      });

    const savedWebhook = Boolean(routine?.triggers.some((trigger) => trigger.type === "webhook"));
    const fireUrl = isEditMode && savedWebhook ? RoutineWebhookHelper.fireUrl(routineId) : null;
    const savedCron = routine?.triggers.find((trigger) => trigger.type === "scheduled");

    async function handleRotate() {
      const confirmed = await confirm({
        title: t("Replace the webhook token?"),
        description: t("The current token stops working immediately. The new one is shown once."),
        confirmText: t("New token"),
        variant: "destructive",
      });
      if (!confirmed) return;

      setRotating(true);
      try {
        const answer = await aos.client.routine.rotate.mutateOrThrow<{ token: string }>({
          params: { routine: routineId },
        });
        setShownToken(answer.token);
      } catch (error) {
        toast.error(t("Failed to replace the token"), { description: errorMessage(error) });
      } finally {
        setRotating(false);
      }
    }

    /**
     * Saves one sidebar field as soon as it is chosen, and puts it back if the
     * daemon refuses: the value on screen used to keep the refused choice, so
     * the page disagreed with the routine and the next Save failed too.
     */
    async function persistField<K extends "status" | "scope">(
      name: K,
      next: RoutineFormValues[K],
      body: Record<string, unknown>,
      successMessage: string,
    ) {
      const previous = form.getValues(name);
      form.setValue(name, next as never);
      if (!isEditMode) return;

      try {
        await aos.client.routine.update.mutateOrThrow({
          params: { routine: routineId },
          body,
        });
        form.resetField(name, { defaultValue: next as never });
        toast.success(successMessage);
        await router.invalidate();
      } catch (error) {
        form.setValue(name, previous as never);
        toast.error(t("Failed to update routine"), { description: errorMessage(error) });
      }
    }

    async function handleStatusChange(status: RoutineStatus) {
      if (status === form.getValues("status")) return;
      await persistField(
        "status",
        status,
        { status },
        t("Status updated to {{status}}", { status: ROUTINE_STATUS_CONFIG[status].label }),
      );
    }

    async function handleScopeChange(patch: Partial<RoutineScope>) {
      const next = { ...form.getValues("scope"), ...patch };
      await persistField(
        "scope",
        next,
        // Merged over what is stored, so an allowlist set elsewhere survives.
        { scope: { ...routine?.scope, ...next } },
        t("Permissions updated."),
      );
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
                      {isEditMode ? routine?.name : t("New Routine")}
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
                                  {isDeleting ? t("Deleting...") : t("Delete routine")}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        ) : null}

                        {isEditMode ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={isFiring}
                                onClick={fireNow}
                              >
                                <PlayIcon />
                                {isFiring
                                  ? t("Running...")
                                  : runsDespiteStatus
                                    ? t("Run once anyway")
                                    : t("Run now")}
                              </Button>
                            </TooltipTrigger>
                            {runsDespiteStatus ? (
                              <TooltipContent sideOffset={8}>
                                {t("This routine is not enabled. This runs it once without enabling it.")}
                              </TooltipContent>
                            ) : null}
                          </Tooltip>
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
                            ? t("Saving...")
                            : isEditMode
                              ? t("Save changes")
                              : t("Create routine")}
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
                        activityEvents={activityEvents}
                        webhook={{
                          fireUrl,
                          onRotate: isEditMode ? () => void handleRotate() : undefined,
                          rotating,
                        }}
                        saved={{
                          cron: savedCron?.type === "scheduled" ? savedCron.config.cron : undefined,
                          nextRun: routine?.nextRun,
                          warnings: RoutineTriggersHelper.describeWarnings(routine),
                        }}
                      />

                      <FormField
                        control={form.control}
                        name="prompt"
                        render={({ field }) => (
                          <FormItem className="space-y-2">
                            <FormLabel className="text-sm font-normal text-muted-foreground">
                              {t("Prompt")}
                            </FormLabel>
                            <FormControl>
                              {/* Plain text, not MarkdownEditor. The prompt is
                                  what the model reads, and a markdown
                                  round-trip rewrites it: `<rules>` saved as
                                  `\<rules>`, `-` bullets as `*`, comments
                                  dropped. What is typed is what is saved. */}
                              <Textarea
                                {...field}
                                value={field.value ?? ""}
                                spellCheck={false}
                                placeholder={t("Enter the system prompt for this routine...")}
                                className="min-h-64 resize-y font-mono text-sm leading-relaxed"
                              />
                            </FormControl>
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
                          onRunNow={fireNow}
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
                              const config = RoutineHelper.getStatus(field.value);
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
                                        <button
                                          type="button"
                                          className="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs hover:bg-accent"
                                        >
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
                                          onStatusChange={(status) => void handleStatusChange(status)}
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
                                  <BotIcon className="size-3.5 shrink-0 text-muted-foreground" />
                                  <span className="w-16 shrink-0 text-xs text-muted-foreground">
                                    {t("Agent")}
                                  </span>

                                  {isEditMode ? (
                                    // A routine lives in its agent's directory
                                    // and Go has no move: every change of owner
                                    // was refused with AOS_ROUTINE_NOT_FOUND.
                                    <Tooltip>
                                      <TooltipTrigger asChild>
                                        <span className="flex items-center gap-1 px-1.5 py-0.5 text-xs">
                                          <Avatar className="size-4">
                                            <AvatarAgentFallback name={currentAgentId.toLowerCase()} />
                                          </Avatar>
                                          <span className="max-w-40 truncate">{agentLabel}</span>
                                        </span>
                                      </TooltipTrigger>
                                      <TooltipContent sideOffset={6}>
                                        {t("A routine belongs to the agent it was created for.")}
                                      </TooltipContent>
                                    </Tooltip>
                                  ) : (
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
                                            {agentLabel || t("Select agent")}
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
                                          onAgentChange={(agent) =>
                                            form.setValue("agent", agent, {
                                              shouldDirty: true,
                                              shouldValidate: true,
                                            })
                                          }
                                        />
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  )}
                                </SplitPageLayout.WidgetItem>
                                <FormMessage className="px-2 text-xs" />
                              </FormItem>
                            )}
                          />
                        </SplitPageLayout.WidgetContent>
                      </SplitPageLayout.Widget>

                      <SplitPageLayout.Widget>
                        <SplitPageLayout.WidgetHeader>
                          <SplitPageLayout.WidgetTitle>
                            {t("Permissions")}
                          </SplitPageLayout.WidgetTitle>
                        </SplitPageLayout.WidgetHeader>
                        <SplitPageLayout.WidgetContent>
                          <FormField
                            control={form.control}
                            name="scope"
                            render={({ field }) => (
                              <FormItem className="flex flex-col gap-2 border-0 p-0">
                                {(
                                  [
                                    ["allowCreateTasks", t("May create tasks")],
                                    ["allowExternalCalls", t("May reach outside this machine")],
                                  ] as const
                                ).map(([key, label]) => (
                                  <label
                                    key={key}
                                    className="flex cursor-pointer items-center gap-2 px-2 py-1 text-xs"
                                  >
                                    <Checkbox
                                      checked={field.value[key]}
                                      onCheckedChange={(checked) =>
                                        void handleScopeChange({ [key]: checked === true })
                                      }
                                    />
                                    <ShieldIcon className="size-3.5 shrink-0 text-muted-foreground" />
                                    <span>{label}</span>
                                  </label>
                                ))}
                                <p className="px-2 text-[11px] text-muted-foreground">
                                  {t("Without these the routine's agent does not see the tools that create tasks or reach the network while it runs.")}
                                </p>
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
          <WebhookTokenDialog
            token={shownToken}
            fireUrl={RoutineWebhookHelper.fireUrl(routineId)}
            onClose={() => setShownToken(null)}
          />
        </PageBody>
      </Page>
    );
  })
  .build();
