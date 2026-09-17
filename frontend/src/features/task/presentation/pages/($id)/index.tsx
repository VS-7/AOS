import { useEffect } from "react";
import { Link, useNavigate, useRouter } from "@tanstack/react-router";
import { SearchX } from "lucide-react";
import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Page, PageBody } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { SplitPageLayout } from "@/components/ui/split-page-layout";
import { TaskDetailsMain } from "./components/main";
import { TaskDetailsSidebar } from "./components/details";
import type { TaskWithContext } from "@/features/task/interfaces/task.interfaces";
import { useTaskActions } from "@/features/task/presentation/hooks/task-actions.hook";
import { useChat } from "@/features/chat/presentation/hooks/use-chat";
import { t } from "@/lib/i18n";

export const TaskDetailsPage = aos.page("/tasks/$id")
  .withMetadata({
    title: "Task Details",
    description: "Task details page",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const result = await client.task.getById.query({ params: { task: request.params.id } });
    // See `(main)/index.tsx`'s loader for why this cast is needed —
    // the facade returns `Envelope<unknown>`, not a typed payload.
    const task = (result.data as { task: TaskWithContext } | undefined)?.task;
    if (task) {
      return { task, missing: null };
    }

    // Only the daemon saying there is no such task is "not found". Every
    // failure used to become the global "Page not found", so a daemon or
    // bridge error read as a wrong address, and a missing task never said it
    // was the task that was missing.
    const code = (result.error as { code?: string } | undefined)?.code;
    if (result.error && code !== "AOS_TASK_NOT_FOUND") {
      throw result.error;
    }
    return { task: null, missing: request.params.id };
  })
  .withComponent(({ route }) => {
    const { task, missing } = route.useLoaderData();
    if (!task) {
      return <TaskNotFound id={missing ?? ""} />;
    }
    return <TaskDetails key={task.id} task={task} />;
  })
  .build();

function TaskDetails({ task }: { task: TaskWithContext }) {
  const router = useRouter();
  const navigate = useNavigate();
  const liveChat = useChat({
    chatId: task.chat ?? "",
    enabled: Boolean(task.chat),
  });
  const actions = useTaskActions(task, {
    onChanged: () => void router.invalidate(),
    onDeleted: () => void navigate({ to: "/tasks" }),
  });

  useEffect(() => {
    aos.stores.viewport.actions.toggle("page.details.visible", true);
  }, []);

  return (
    <Page className="h-full overflow-hidden">
      <PageBody className="overflow-hidden">
        <SplitPageLayout>
          <SplitPageLayout.Content>
            <TaskDetailsMain task={task} actions={actions} liveChat={liveChat} />
          </SplitPageLayout.Content>

          <SplitPageLayout.Detail>
            <TaskDetailsSidebar task={task} actions={actions} liveChat={liveChat} />
          </SplitPageLayout.Detail>
        </SplitPageLayout>
      </PageBody>
    </Page>
  );
}

function TaskNotFound({ id }: { id: string }) {
  return (
    <Page className="h-full overflow-hidden">
      <PageBody className="flex items-center justify-center">
        <div className="flex max-w-md flex-col items-center gap-3 px-6 text-center">
          <SearchX className="size-8 text-muted-foreground" />
          <h1 className="text-base font-semibold">{t("Task not found")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("No task with the id {{id}} exists in this workspace. It may have been deleted.", { id })}
          </p>
          <Button asChild variant="outline" size="sm">
            <Link to="/tasks">{t("Back to tasks")}</Link>
          </Button>
        </div>
      </PageBody>
    </Page>
  );
}
