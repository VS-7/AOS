import * as React from "react";
import { useRouter, useNavigate } from "@tanstack/react-router";
import { AnimatePresence, motion } from "framer-motion";
import { toast } from "sonner";
import { z } from "zod";
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
import { Form } from "@/components/ui/form";
import { Kbd } from "@/components/ui/kbd";
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
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import {
  ProjectStatusSchema,
  type Project,
} from "@/features/project/interfaces/project.interfaces";
import { ProjectIconPicker } from "@/features/project/presentation/components/project-icon-picker";
import { ProjectDetailsSidebar } from "./components/details";
import { ProjectOverviewTab } from "./components/main/components/tabs/overview";
import { ProjectTasksTab } from "./components/main/components/tabs/tasks";
import { ProjectGoalsTab } from "./components/main/components/tabs/goals";
import { ProjectFilesTab } from "./components/main/components/tabs/files";
import { t, useTranslation } from "@/lib/i18n";
import { shareableLink } from "@/features/goal/presentation/helpers/goal-link";
import {
  hasSluggableCharacter,
  isAbsoluteSource,
  projectCreateBody,
  projectIdPreview,
  projectUpdateBody,
} from "@/features/project/presentation/helpers/project-form";
import {
  Copy,
  FileText,
  Folder,
  FolderOpen,
  Goal,
  Link2,
  ListChecks,
  Save,
  Trash2,
} from "lucide-react";

// The messages are resolved when the schema validates, so they follow the
// language of that moment. Both rules are the daemon's: it derives the id from
// the name, and binds a project only to an absolute directory — the form says
// so on the field instead of sending what will be refused.
const projectFormSchema = z.object({
  name: z
    .string()
    .trim()
    .superRefine((value, ctx) => {
      if (!value) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, message: t("Name is required") });
      } else if (!hasSluggableCharacter(value)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("Use at least one letter or number in the name"),
        });
      }
    }),
  icon: z.string().optional(),
  description: z.string().optional(),
  content: z.string().optional(),
  source: z
    .string()
    .optional()
    .superRefine((value, ctx) => {
      if (value?.trim() && !isAbsoluteSource(value.trim())) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("Use an absolute path, such as /Users/you/project"),
        });
      }
    }),
  status: ProjectStatusSchema.optional(),
});

type ProjectFormValues = z.infer<typeof projectFormSchema>;

interface HeaderIconButtonProps {
  children: React.ReactNode;
  label: string;
  shortcut: string;
  onClick?: () => void | Promise<void>;
}

function HeaderIconButton({
  children,
  label,
  shortcut,
  onClick,
}: HeaderIconButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-8 rounded-md"
          onClick={onClick}
        >
          {children}
          <span className="sr-only">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent sideOffset={8} className="flex items-center gap-2">
        <span>{label}</span>
        <Kbd>{shortcut}</Kbd>
      </TooltipContent>
    </Tooltip>
  );
}

function getErrorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  return t("Unable to save this project.");
}

function buildFormValues(project: Project | null): ProjectFormValues {
  if (!project) {
    return {
      name: "",
      icon: "",
      description: "",
      content: "",
      source: "",
      status: "active",
    };
  }

  return {
    name: project.name,
    icon: project.icon ?? "",
    description: project.description ?? "",
    content: project.content ?? "",
    source: project.source ?? "",
    status: project.status ?? "active",
  };
}

interface ProjectCreateSidebarProps {
  values: ProjectFormValues;
}

function ProjectCreateSidebar({ values }: ProjectCreateSidebarProps) {
  const previewId = projectIdPreview(values.name ?? "");

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label={t("Overview")}>
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>{t("Preview")}</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <SplitPageLayout.WidgetItem>
              <Folder className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                {t("Route")}
              </span>
              <span className="font-mono text-xs text-muted-foreground">
                /projects/{previewId}
              </span>
            </SplitPageLayout.WidgetItem>

            {values.source?.trim() ? (
              <SplitPageLayout.WidgetItem>
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-16 shrink-0 text-xs text-muted-foreground">
                  {t("Source")}
                </span>
                <span
                  className="truncate text-xs text-muted-foreground"
                  title={values.source}
                >
                  {values.source}
                </span>
              </SplitPageLayout.WidgetItem>
            ) : null}
          </SplitPageLayout.WidgetContent>
        </SplitPageLayout.Widget>

        {values.description?.trim() ? (
          <SplitPageLayout.Widget>
            <SplitPageLayout.WidgetHeader>
              <SplitPageLayout.WidgetTitle>
                {t("Description")}
              </SplitPageLayout.WidgetTitle>
            </SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetContent>
              <SplitPageLayout.WidgetItem>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {values.description}
                </p>
              </SplitPageLayout.WidgetItem>
            </SplitPageLayout.WidgetContent>
          </SplitPageLayout.Widget>
        ) : null}
      </SplitPageLayout.DetailTab>
    </SplitPageLayout.DetailTabs>
  );
}

export const ProjectDetailsPage = aos
  .page("/projects/$id")
  .withMetadata({
    title: "Project",
    description: "Create and edit projects",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const isCreate = request.params.id === "new";

    // Task 10: the `project` domain is dormant — see the matching comment
    // in `goal/presentation/pages/($id)/index.tsx` for the reasoning.
    if (isDormant("project") || isCreate) {
      return {
        mode: "create" as const,
        project: null as Project | null,
      };
    }

    const result = await client.project.getById.query({
      params: { project: request.params.id },
    });
    const project = result.data?.project;

    if (!project) {
      return response.notFound();
    }

    return {
      mode: "edit" as const,
      project,
    };
  })
  .withComponent(({ route }) => {
    const router = useRouter();
    const navigate = useNavigate();
    const { t } = useTranslation();
    const { mode, project } = route.useLoaderData();
    const projectId = route.useParams().id;
    const isEditMode = mode === "edit";
    const [selectedIndex, setSelectedIndex] = React.useState(0);

    React.useEffect(() => {
      aos.stores.viewport.actions.toggle("page.details.visible", true);
    }, []);

    const form = aos.useForm({
      schema: projectFormSchema,
      values: buildFormValues(project),
      onSubmit: async (values: ProjectFormValues) => {
        // A refusal about one field belongs on that field, where the person
        // will fix it; anything else is a toast carrying the daemon's reason.
        const placeOnField = (error: unknown) => {
          const code = (error as { code?: string } | undefined)?.code;
          if (code === "AOS_PROJECT_ALREADY_EXISTS") {
            form.setError("name", {
              message: t("A project with this name already exists. Choose another name."),
            });
            return true;
          }
          if (code === "AOS_PROJECT_NAME_REQUIRED") {
            form.setError("name", { message: t("Use at least one letter or number in the name") });
            return true;
          }
          if (code === "AOS_PROJECT_SOURCE_INVALID") {
            form.setError("source", {
              message: t("This directory does not exist on this machine, or is not a directory."),
            });
            return true;
          }
          return false;
        };

        if (isEditMode && project) {
          const result = await aos.client.project.update.mutate({
            params: { project: projectId },
            // Cleared fields go as "", which Go reads as clear; they used to
            // go as undefined, and the source could never be unbound.
            body: projectUpdateBody(values),
          });

          if (result?.error) {
            if (placeOnField(result.error)) return;
            toast.error(t("Could not save the project"), {
              description: getErrorMessage(result.error),
            });
            return;
          }

          toast.success(t("Project updated."));
          void aos.stores.projects.actions.refresh();
          router.invalidate();
          return;
        }

        const result = await aos.client.project.create.mutate({
          body: projectCreateBody(values),
        });

        if (result?.error || !result.data?.project?.id) {
          if (placeOnField(result?.error)) return;
          toast.error(t("Could not create the project"), {
            description: getErrorMessage(result?.error),
          });
          return;
        }

        toast.success(t("Project created."));
        void aos.stores.projects.actions.refresh();
        await router.invalidate();
        await navigate({
          to: "/projects/$id",
          params: { id: result.data.project.id },
        });
      },
    });

    // [Reset form]: When navigating between projects, defaultValues alone don't trigger
    // a form reset because react-hook-form only applies defaultValues on mount.
    // We must explicitly reset when the project ID changes.
    React.useEffect(() => {
      form.reset(buildFormValues(project));
      setSelectedIndex(0);
    }, [projectId]);

    const { mutate: deleteProject, loading: isDeleting } =
      aos.client.project.delete.useMutation({
        onSuccess: async () => {
          toast.success(t("Project deleted."));
          // Leave first. Invalidating while still on /projects/$id reran this
          // page's loader for the project just removed, which answered
          // PROJECT_NOT_FOUND before the navigation happened.
          await navigate({ to: "/projects", replace: true });
          void aos.stores.projects.actions.refresh();
        },
        onError: (error: unknown) => {
          toast.error(t("Could not delete the project"), {
            description: getErrorMessage(error),
          });
        },
      });

    const watchedValues = form.watch();
    const liveProject = React.useMemo<Project | null>(() => {
      if (!project) return null;

      return {
        ...project,
        name: watchedValues.name?.trim() || project.name,
        icon: watchedValues.icon?.trim() || undefined,
        description: watchedValues.description?.trim() || undefined,
        content: watchedValues.content?.trim() || undefined,
        source: watchedValues.source?.trim() || undefined,
        status: watchedValues.status ?? project.status,
      };
    }, [project, watchedValues]);

    const link =
      isEditMode && project ? shareableLink(`/projects/${project.id}`) : null;

    async function copyToClipboard(value: string, message: string) {
      await navigator.clipboard.writeText(value);
      toast.success(message);
    }

    const editProject = liveProject ?? project;
    const tabsIdPrefix = `project-upsert-${project?.id ?? "new"}`;

    return (
      <DormantGate feature="project">
      <Page className="h-full overflow-hidden">
        <PageBody className="overflow-hidden">
          <Form form={form} className="flex h-full flex-1 flex-col">
            <SplitPageLayout>
              <SplitPageLayout.Content>
                <div className="grid h-full grid-rows-[auto_1fr]">
                  <SplitPageLayout.ContentHeader>
                    <SplitPageLayout.ContentHeaderMain className="items-center gap-3 -ml-1">
                      <ProjectIconPicker
                        value={watchedValues.icon}
                        onChange={(next) =>
                          form.setValue("icon", next, {
                            shouldDirty: true,
                          })
                        }
                        disabled={form.isLoading}
                      />
                      <SplitPageLayout.ContentTitle>
                        {isEditMode ? editProject?.name : t("New Project")}
                      </SplitPageLayout.ContentTitle>
                    </SplitPageLayout.ContentHeaderMain>

                    <SplitPageLayout.ContentHeaderActions>
                      <TooltipProvider>
                        <div className="flex items-center gap-2">
                          {isEditMode && link && project ? (
                            <>
                              <HeaderIconButton
                                label={link.label}
                                shortcut="L"
                                onClick={() =>
                                  copyToClipboard(link.value, link.copied)
                                }
                              >
                                <Link2 />
                              </HeaderIconButton>
                              <HeaderIconButton
                                label={t("Copy project ID")}
                                shortcut="I"
                                onClick={() =>
                                  copyToClipboard(project.id, t("Project ID copied"))
                                }
                              >
                                <Copy />
                              </HeaderIconButton>
                            </>
                          ) : null}

                          {isEditMode && project ? (
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
                                        {t("Delete project")}
                                      </span>
                                    </Button>
                                  </TooltipTrigger>
                                </AlertDialogTrigger>
                                <TooltipContent sideOffset={8}>
                                  {t("Delete project")}
                                </TooltipContent>
                              </Tooltip>
                              <AlertDialogContent size="sm">
                                <AlertDialogHeader>
                                  <AlertDialogTitle>
                                    {t("Delete this project?")}
                                  </AlertDialogTitle>
                                  <AlertDialogDescription>
                                    {t("This permanently removes {{name}}. Its tasks and goals are kept, without the project.", {
                                      name: project.name,
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
                                      deleteProject({
                                        params: { project: project.id },
                                      })
                                    }
                                  >
                                    {isDeleting
                                      ? t("Deleting...")
                                      : t("Delete project")}
                                  </AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
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
                                : t("Create project")}
                          </Button>
                        </div>
                      </TooltipProvider>
                    </SplitPageLayout.ContentHeaderActions>
                  </SplitPageLayout.ContentHeader>

                  <SplitPageLayout.ContentBody>
                    {isEditMode && editProject ? (
                      <div className="grid h-full grid-rows-[auto_1fr] overflow-hidden">
                        <div className="px-4 pt-2">
                          <TabsSubtle
                            selectedIndex={selectedIndex}
                            onSelect={setSelectedIndex}
                            idPrefix={tabsIdPrefix}
                            activeLabel
                          >
                            <TabsSubtleItem
                              index={0}
                              icon={FileText}
                              label={t("Overview")}
                            />
                            <TabsSubtleItem
                              index={1}
                              icon={ListChecks}
                              label={t("Tasks")}
                            />
                            <TabsSubtleItem
                              index={2}
                              icon={Goal}
                              label={t("Goals")}
                            />
                            <TabsSubtleItem
                              index={3}
                              icon={FolderOpen}
                              label={t("Files")}
                            />
                          </TabsSubtle>
                        </div>

                        <div className="overflow-auto">
                          <TabsSubtlePanel
                            index={0}
                            selectedIndex={selectedIndex}
                            idPrefix={tabsIdPrefix}
                            className="h-full"
                          >
                            <ProjectOverviewTab
                              form={form}
                              project={editProject}
                            />
                          </TabsSubtlePanel>
                          <TabsSubtlePanel
                            index={1}
                            selectedIndex={selectedIndex}
                            idPrefix={tabsIdPrefix}
                            className="h-full"
                          >
                            <ProjectTasksTab project={editProject} />
                          </TabsSubtlePanel>
                          <TabsSubtlePanel
                            index={2}
                            selectedIndex={selectedIndex}
                            idPrefix={tabsIdPrefix}
                            className="h-full"
                          >
                            <ProjectGoalsTab project={editProject} />
                          </TabsSubtlePanel>
                          <TabsSubtlePanel
                            index={3}
                            selectedIndex={selectedIndex}
                            idPrefix={tabsIdPrefix}
                            className="h-full"
                          >
                            <ProjectFilesTab project={editProject} />
                          </TabsSubtlePanel>
                        </div>
                      </div>
                    ) : (
                      <ProjectOverviewTab form={form} />
                    )}
                  </SplitPageLayout.ContentBody>
                </div>
              </SplitPageLayout.Content>

              <SplitPageLayout.Detail>
                {isEditMode && editProject ? (
                  <ProjectDetailsSidebar project={editProject} form={form} />
                ) : (
                  <ProjectCreateSidebar values={watchedValues} />
                )}
              </SplitPageLayout.Detail>
            </SplitPageLayout>
          </Form>
        </PageBody>
      </Page>
      </DormantGate>
    );
  })
  .build();
