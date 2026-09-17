import type { ActivityEventDefinition } from "@/features/activity/interfaces/activity.interfaces";
import type { Routine, Run } from "@/features/routine/interfaces/routine.interfaces";

/**
 * What the routine page reads before it renders.
 *
 * It lives beside the page rather than inside `withLoader` because the order
 * of these reads is the behaviour: a routine that is gone must be reported
 * once, not once per command the page happened to send.
 */
export interface RoutinePageLoaderArgs {
  client: any;
  request: { params: Record<string, string> };
  // `never`: the router's own not-found is thrown, not returned (see
  // app/builders/response.ts), which is why the calls below `return` it.
  response: { notFound: () => never };
}

export interface RoutinePageData {
  mode: "create" | "edit";
  routine: Routine | null;
  runs: Run[];
  activityEvents: ActivityEventDefinition[];
}

export async function loadRoutinePage({
  client,
  request,
  response,
}: RoutinePageLoaderArgs): Promise<RoutinePageData> {
  const isCreate = request.params.id === "new";

  // No filter: the catalogue *is* the set of events a routine can react to,
  // so `routine: true` named a distinction Go does not make.
  const eventsResult = await client.activity.listEvents.query({});
  const activityEvents = (eventsResult.data ?? []) as ActivityEventDefinition[];

  if (isCreate) {
    return {
      mode: "create" as const,
      routine: null as Routine | null,
      runs: [] as Run[],
      activityEvents,
    };
  }

  // The run history is its own command: `routines_get` answers the routine
  // alone, and the page read `routine.runs`, which Go has never carried — so
  // the history said "No runs yet" beside every run on disk.
  //
  // The routine is read first, and its history only once it exists. Asked
  // together, opening a routine somebody had just deleted refused both and
  // wrote two NOT_FOUND errors to the log for one missing routine, the second
  // of which added nothing to the first.
  const result = await client.routine.getById.query({
    params: { routine: request.params.id },
  });

  const routine = result.data?.routine as Routine | undefined;

  if (!routine) {
    return response.notFound();
  }

  const runsResult = await client.routine.runs.query({
    params: { routine: request.params.id },
    query: { limit: 200 },
  });

  return {
    mode: "edit" as const,
    routine,
    runs: (runsResult.data?.runs ?? []) as Run[],
    activityEvents,
  };
}
