import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";
import type { Task } from "@/features/task/interfaces/task.interfaces";

import { TasksProvider } from "./context";
import { TasksPageInner } from "./inner";

const TasksPageSearchSchema = Schema.object({
  view: z.enum(["list", "kanban"]).optional(),
  query: z.string().optional(),
  status: z.string().optional(),
  priority: z.string().optional(),
  type: z.string().optional(),
  project: z.string().optional(),
  goal: z.string().optional(),
});

export const TasksPage = aos
  .page("/tasks")
  .withMetadata({
    title: "Tasks",
    description: "Task listing workspace",
  })
  .withQuery(TasksPageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const query = request.query || {};

    // Only the search goes to the daemon. Priority, type, project and goal
    // are applied by the page (`TasksProvider`, `filterTasks`): tasks_list
    // takes one value for each, so a priority array was refused and a second
    // type or project was ignored.
    const response = await client.task.list.query({
      query: { query: query.query?.trim() || undefined },
    });

    // A refused or failed read used to become an empty list, which is
    // indistinguishable from a workspace with no tasks. Thrown, it reaches
    // the route's error screen with the daemon's reason and a retry.
    if (response.error) throw response.error;

    // The facade's `query()` deliberately returns `Envelope<unknown>` (see
    // `lib/aos-facade.ts`) — the ported code assumed a strongly-typed RPC
    // client, so every call site that reads `.data` needs this cast; Go's
    // own input/output validation is the real safety net, per the
    // facade's own docs.
    const data = response.data as { tasks: Task[] } | undefined;
    const tasks = data?.tasks || [];
    return { tasks };
  })
  .withComponent(({ route, client }) => {
    const { tasks } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <TasksProvider
        tasks={tasks}
        search={search}
        client={client}
        route={route}
      >
        <TasksPageInner />
      </TasksProvider>
    );
  })
  .build();
