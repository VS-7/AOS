import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Schema } from "@/core/helpers/schema.helper";
import { z } from "zod";

import type { Routine } from "@/features/routine/interfaces/routine.interfaces";
import { RoutinesProvider } from "./context";
import { RoutinesPageInner } from "./inner";

const RoutinesPageSearchSchema = Schema.object({
  query: z.string().optional(),
  status: z.string().optional(),
  agent: z.string().optional(),
  type: z.string().optional(),
});

/** More than any workspace keeps; the page has no pagination to offer. */
const LIST_LIMIT = 300;

export const RoutinesPage = aos
  .page("/routines")
  .withMetadata({
    title: "Routines",
    description: "Routines listing workspace",
  })
  .withQuery(RoutinesPageSearchSchema)
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client }) => {
    // Every routine, filtered on this side. Search and status used to be
    // forwarded: `limit` went as the string "300" and Go refused the whole
    // list, a multi-status filter reached Go as "enabled,disabled" and matched
    // nothing, and each keystroke re-ran this loader.
    const [response, events] = await Promise.all([
      client.routine.list.query({ query: { limit: LIST_LIMIT } }),
      client.activity.listEvents.query({}),
    ]);

    return {
      routines: (response.data?.routines || []) as Routine[],
      activityEvents: events.data ?? [],
    };
  })
  .withComponent(({ route }) => {
    const { routines, activityEvents } = route.useLoaderData();
    const search = route.useSearch();

    return (
      <RoutinesProvider routines={routines} activityEvents={activityEvents} search={search}>
        <RoutinesPageInner />
      </RoutinesProvider>
    );
  })
  .build();
