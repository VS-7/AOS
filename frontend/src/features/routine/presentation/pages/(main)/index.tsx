import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";

import { RoutinesProvider } from "./context";
import { RoutinesPageInner } from "./inner";

const RoutinesPageSearchSchema = Schema.object({
  query: z.string().optional(),
  status: z.string().optional(),
  agent: z.string().optional(),
  type: z.string().optional(),
});

export const RoutinesPage = aos
  .page("/routines")
  .withMetadata({
    title: "Routines",
    description: "Routines listing workspace",
  })
  .withQuery(RoutinesPageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request }) => {
    const query = request.query || {};

    const response = await client.routine.list.query({
      query: {
        query: query.query?.trim() || undefined,
        status: query.status || undefined,
        limit: "300",
      },
    });

    return { routines: response.data?.routines || [] };
  })
  .withComponent(({ route }) => {
    const { routines } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <RoutinesProvider routines={routines} search={search}>
        <RoutinesPageInner />
      </RoutinesProvider>
    );
  })
  .build();
