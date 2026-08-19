import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";

import { GoalsProvider } from "./context";
import { GoalsPageInner } from "./inner";

const GoalsPageSearchSchema = Schema.object({
  query: z.string().optional(),
  status: z.string().optional(),
  priority: z.string().optional(),
  project: z.string().optional(),
});

export const GoalsPage = aos
  .page("/goals")
  .withMetadata({
    title: "Goals",
    description: "Goal listing workspace",
  })
  .withQuery(GoalsPageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const query = request.query || {};

    const response = await client.goal.list.query({
      query: {
        query: query.query?.trim() || undefined,
        status: query.status as any,
        project: query.project || undefined,
      },
    });

    const goals = response.data?.goals || [];
    return { goals };
  })
  .withComponent(({ route, client }) => {
    const { goals } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <GoalsProvider goals={goals} search={search}>
        <GoalsPageInner />
      </GoalsProvider>
    );
  })
  .build();
