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
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import type { FractalProject } from "@/features/project/interfaces/project.interfaces";
import { ProjectIconPicker } from "@/features/project/presentation/components/project-icon-picker";
import { ProjectDetailsSidebar } from "./components/details";
import { ProjectOverviewTab } from "./components/main/components/tabs/overview";
import { ProjectTasksTab } from "./components/main/components/tabs/tasks";
import { ProjectGoalsTab } from "./components/main/components/tabs/goals";
import { ProjectFilesTab } from "./components/main/components/tabs/files";
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

const projectFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required"),
  icon: z.string().optional(),
  description: z.string().optional(),
  content: z.string().optional(),
  source: z.string().optional(),
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
  return "Unable to save this project.";
}

function buildFormValues(project: FractalProject | null): ProjectFormValues {
  if (!project) {
    return {
      name: "",
      icon: "",
      description: "",
      content: "",
      source: "",
    };
  }

  return {
    name: project.name,
    icon: project.icon ?? "",
    description: project.description ?? "",
    content: project.content ?? "",
    source: project.source ?? "",
  };
}

function buildPreviewProjectId(name: string) {
  const previewId = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  return previewId || "project-id";
}

function buildProjectPayload(values: ProjectFormValues) {
  return {
    name: values.name.trim(),
    // Empty string clears a previous custom icon/image on update.
    icon: (values.icon ?? "").trim(),
    description: values.description?.trim() || undefined,
    content: values.content?.trim() || undefined,
    source: values.source?.trim() || undefined,
  };
}

interface ProjectCreateSidebarProps {
  values: ProjectFormValues;
}

function ProjectCreateSidebar({ values }: ProjectCreateSidebarProps) {
  const previewId = buildPreviewProjectId(values.name);

  return (
    <SplitPageLayout.DetailTabs defaultValue="overview">
      <SplitPageLayout.DetailTab value="overview" label="Overview">
        <SplitPageLayout.Widget>
          <SplitPageLayout.WidgetHeader>
            <SplitPageLayout.WidgetTitle>Preview</SplitPageLayout.WidgetTitle>
          </SplitPageLayout.WidgetHeader>
          <SplitPageLayout.WidgetContent>
            <SplitPageLayout.WidgetItem>
              <Folder className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="w-16 shrink-0 text-xs text-muted-foreground">
                Route
              </span>
              <span className="font-mono text-xs text-muted-foreground">
                /projects/{previewId}
              </span>
            </SplitPageLayout.WidgetItem>

            {values.source?.trim() ? (
              <SplitPageLayout.WidgetItem>
                <Folder className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="w-16 shrink-0 text-xs text-muted-foreground">
                  Source
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
                Description
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

    if (isCreate) {
      return {
        mode: "create" as const,
        project: null as FractalProject | null,
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
        const body = buildProjectPayload(values);

        if (isEditMode && project) {
          const result = await aos.client.project.update.mutate({
            params: { project: projectId },
            body,
          });

          if (result?.error) {
            toast.error(getErrorMessage(result.error));
            return;
          }

          toast.success("Project updated.");
          void aos.stores.projects.actions.refresh();
          router.invalidate();
          return;
        }

        const result = await aos.client.project.create.mutate({ body });

        if (result?.error || !result.data?.project?.id) {
          toast.error(getErrorMessage(result?.error));
          return;
        }

        toast.success("Project created.");
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
          toast.success("Project deleted.");
          void aos.stores.projects.actions.refresh();
          await router.invalidate();
          await navigate({ to: "/projects" });
        },
        onError: (error: unknown) => {
          toast.error(getErrorMessage(error));
        },
      });

    const watchedValues = form.watch();
    const liveProject = React.useMemo<FractalProject | null>(() => {
      if (!project) return null;

      return {
        ...project,
        name: watchedValues.name?.trim() || project.name,
        icon: watchedValues.icon?.trim() || undefined,
        description: watchedValues.description?.trim() || undefined,
        content: watchedValues.content?.trim() || undefined,
        source: watchedValues.source?.trim() || undefined,
      };
    }, [project, watchedValues]);

    const projectLink =
      isEditMode && project
        ? typeof window === "undefined"
          ? `/projects/${project.id}`
          : new URL(
              `/projects/${project.id}`,
              window.location.origin,
            ).toString()
        : null;

    async function copyToClipboard(value: string, label: string) {
      await navigator.clipboard.writeText(value);
      toast.success(`${label} copied`);
    }

    const editProject = liveProject ?? project;
    const tabsIdPrefix = `project-upsert-${project?.id ?? "new"}`;

    return (
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
                        {isEditMode ? editProject?.name : "New Project"}
                      </SplitPageLayout.ContentTitle>
                    </SplitPageLayout.ContentHeaderMain>

                    <SplitPageLayout.ContentHeaderActions>
                      <TooltipProvider>
                        <div className="flex items-center gap-2">
                          {isEditMode && projectLink && project ? (
                            <>
                              <HeaderIconButton
                                label="Copy project link"
                                shortcut="L"
                                onClick={() =>
                                  copyToClipboard(projectLink, "Project link")
                                }
                              >
                                <Link2 />
                              </HeaderIconButton>
                              <HeaderIconButton
                                label="Copy project ID"
                                shortcut="I"
                                onClick={() =>
                                  copyToClipboard(project.id, "Project ID")
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
                                        Delete project
                                      </span>
                                    </Button>
                                  </TooltipTrigger>
                                </AlertDialogTrigger>
                                <TooltipContent sideOffset={8}>
                                  Delete project
                                </TooltipContent>
                              </Tooltip>
                              <AlertDialogContent size="sm">
                                <AlertDialogHeader>
                                  <AlertDialogTitle>
                                    Delete this project?
                                  </AlertDialogTitle>
                                  <AlertDialogDescription>
                                    This action removes{" "}
                                    <strong>{project.name}</strong> permanently.
                                  </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel disabled={isDeleting}>
                                    Cancel
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
                                      ? "Deleting..."
                                      : "Delete project"}
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
                              ? "Saving..."
                              : isEditMode
                                ? "Save changes"
                                : "Create project"}
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
                              label="Overview"
                            />
                            <TabsSubtleItem
                              index={1}
                              icon={ListChecks}
                              label="Tasks"
                            />
                            <TabsSubtleItem
                              index={2}
                              icon={Goal}
                              label="Goals"
                            />
                            <TabsSubtleItem
                              index={3}
                              icon={FolderOpen}
                              label="Files"
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
                  <ProjectDetailsSidebar project={editProject} />
                ) : (
                  <ProjectCreateSidebar values={watchedValues} />
                )}
              </SplitPageLayout.Detail>
            </SplitPageLayout>
          </Form>
        </PageBody>
      </Page>
    );
  })
  .build();
