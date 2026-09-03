import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Page, PageBody } from "@/components/ui/page";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { TaskDetailsMain } from "./components/main";
import { TaskDetailsSidebar } from "./components/details";
import type { TaskWithContext, TaskPriority } from "@/features/task/interfaces/task.interfaces";
import { toast } from "sonner";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { TaskHelper } from "@/features/task/presentation/helpers/task.helper";
import { useTasksStatusTransition } from "@/features/task/presentation/hooks/tasks-status-transition.hook";
import { TasksFinishWorkflowDialog } from "@/features/task/presentation/components/dialogs/finish";
import { useEffect } from "react";
import { useChat } from "@/features/chat/presentation/hooks/use-chat";
import { t } from "@/lib/i18n";

export const TaskDetailsPage = aos.page("/tasks/$id")
  .withMetadata({
    title: "Task Details",
    description: "Task details page",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    const result = await client.task.getById.query({ params: { task: request.params.id } });
    // See `(main)/index.tsx`'s loader for why this cast is needed —
    // the facade returns `Envelope<unknown>`, not a typed payload.
    const task = (result.data as { task: TaskWithContext } | undefined)?.task;

    if (!task) {
      return response.notFound();
    }

    return { task };
  })
  .withComponent(({ route, client }) => {
    const { task } = route.useLoaderData();
    const liveChat = useChat({
      chatId: task.chat ?? "",
      enabled: Boolean(task.chat),
    });

    const router = useRouter();
    const finishTransition = useTasksStatusTransition();

    useEffect(() => {
      aos.stores.viewport.actions.toggle("page.details.visible", true);
    }, []);

    async function handleStatusChange(status: TaskWithContext["status"]) {
      if (status === task.status) return;
      if (status === "finished") {
        finishTransition.open(task, status);
        return;
      }
      const { error } = await aos.client.task.setStatus.mutate({
        params: { task: task.id },
        body: { status },
      });
      if (error) {
        // @ts-expect-error - Expected
        toast.error(error.error?.message || "Failed to update status");
        return;
      }
      toast.success(`Moved to ${TaskHelper.getStatus(status).label}`);
      router.invalidate();
    }

    async function handlePriorityChange(priority: TaskPriority) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { priority } });
        toast.success(t("Priority updated"));
        router.invalidate();
      } catch {
        toast.error(t("Failed to update priority"));
      }
    }

    async function handleAssigneeChange(assignee: string | undefined) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { assigned: assignee } });
        toast.success(assignee ? "Assigned" : "Unassigned");
        router.invalidate();
      } catch {
        toast.error(t("Failed to update assignee"));
      }
    }

    async function handleTypeChange(type: string) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { type } });
        toast.success(t("Type updated"));
        router.invalidate();
      } catch {
        toast.error(t("Failed to update type"));
      }
    }

    async function handleDueDateChange(dueAt: string | undefined) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { dueAt } });
        toast.success(dueAt ? "Due date set" : "Due date removed");
        router.invalidate();
      } catch {
        toast.error(t("Failed to update due date"));
      }
    }

    async function handleProjectChange(project: string | undefined) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { project } });
        toast.success(project ? "Project updated" : "Project cleared");
        router.invalidate();
      } catch {
        toast.error(t("Failed to update project"));
      }
    }

    async function handleGoalChange(goal: string | undefined) {
      try {
        await aos.client.task.update.mutateOrThrow({ params: { task: task.id }, body: { goal } });
        toast.success(goal ? "Goal updated" : "Goal cleared");
        router.invalidate();
      } catch {
        toast.error(t("Failed to update goal"));
      }
    }

    return (
      <>
        <Page className="h-full overflow-hidden">
          <PageBody className="overflow-hidden">
            <SplitPageLayout>
              <SplitPageLayout.Content>
                <TaskDetailsMain
                  client={aos.client}
                  liveChat={liveChat}
                  refresh={route.refresh}
                  task={task}
                />
              </SplitPageLayout.Content>

              <SplitPageLayout.Detail>
                <TaskDetailsSidebar
                  liveChat={liveChat}
                  task={task}
                  onStatusChange={handleStatusChange}
                  onPriorityChange={handlePriorityChange}
                  onTypeChange={handleTypeChange}
                  onAssigneeChange={handleAssigneeChange}
                  onDueDateChange={handleDueDateChange}
                  onProjectChange={handleProjectChange}
                  onGoalChange={handleGoalChange}
                />
              </SplitPageLayout.Detail>
            </SplitPageLayout>
          </PageBody>
        </Page>

        <TasksFinishWorkflowDialog
          open={finishTransition.state.open}
          task={finishTransition.state.task}
          onOpenChange={(open) => {
            if (!open) finishTransition.close();
          }}
          onConfirm={async (input) => {
            if (!finishTransition.state.task) {
              finishTransition.close();
              return;
            }

            const { error } = await aos.client.task.setStatus.mutate({
              params: { task: finishTransition.state.task.id },
              body: input,
            });

            if (error) {
              // @ts-expect-error - Expected
              toast.error(error.error?.message || "Failed to finish task");
              return;
            }

            toast.success(`Finished ${finishTransition.state.task.id}`);
            finishTransition.close();
            router.invalidate();
          }}
        />
      </>
    );
  })
  .build();
