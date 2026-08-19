import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";
import type { FractalTask, FractalTaskPriority } from "@/features/task/interfaces/task.interfaces";

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

function parseMultiValue(value?: string): string[] {
  if (!value) return [];
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseOptionalMultiValue(value?: string): string[] | undefined {
  const values = parseMultiValue(value);
  return values.length > 0 ? values : undefined;
}

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
    const priority = parseOptionalMultiValue(query.priority) as
      | FractalTaskPriority[]
      | undefined;
    const type = parseOptionalMultiValue(query.type);
    const project = parseOptionalMultiValue(query.project);
    const goal = parseOptionalMultiValue(query.goal);

    const response = await client.task.list.query({
      query: {
        query: query.query?.trim() || undefined,
        ...(priority ? { priority } : {}),
        ...(type ? { type } : {}),
        ...(project ? { project } : {}),
        ...(goal ? { goal } : {}),
      },
    });

    // The facade's `query()` deliberately returns `Envelope<unknown>` (see
    // `lib/aos-facade.ts`) — the ported code assumed a strongly-typed RPC
    // client, so every call site that reads `.data` needs this cast; Go's
    // own input/output validation is the real safety net, per the
    // facade's own docs.
    const data = response.data as { tasks: FractalTask[] } | undefined;
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
