import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";
import { DormantGate } from "@/components/DormantDomain";

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
  .withLoader(async ({ client }) => {
    // Every goal, unfiltered: the filters live in the URL and are applied by
    // GoalsProvider, which can do what goals_list cannot — several projects
    // at once, and text that matches a description. Forwarding them sent the
    // comma-joined status string Go refused, a comma-joined project Go
    // compared as one id, and a search Go ran over id and title only, so each
    // emptied the list before the provider saw it.
    const response = await client.goal.list.query({ query: {} });

    const goals = response.data?.goals || [];
    return { goals };
  })
  .withComponent(({ route, client }) => {
    const { goals } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <DormantGate feature="goal">
        <GoalsProvider goals={goals} search={search}>
          <GoalsPageInner />
        </GoalsProvider>
      </DormantGate>
    );
  })
  .build();
