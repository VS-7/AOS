import { useEffect, useMemo } from "react";
import { aos } from "@/app/aos";
import type { AssigneeDirectoryInput } from "@/features/task/presentation/helpers/assignee.helper";
import type { WorkspaceDirectoryAgent } from "@/features/workspace/interfaces/directory.interfaces";

// Asked once per window while the request is in flight or has succeeded: an
// installation whose account list really is empty would otherwise ask again
// every time a picker mounted. A failure clears it again — the flag was set
// before the call and never reset, so one transient bridge error at start-up
// (the always-mounted task dialog makes this request on every load) left
// every picker showing only the signed-in person until the window reloaded.
let peopleRequested = false;

/**
 * Who a task can be assigned to, and who a task's assignee id names.
 *
 * Every picker read `stores.workspace.directory`, which nothing fills at
 * start: only the sidebar's Team tab and the composer call
 * `refreshDirectory`. On a fresh load the Agents section was empty, no task
 * could be given to an agent, and an agent-owned row showed initials instead
 * of the agent's avatar. The agent store is the roster that is preloaded and
 * refreshed on every agent change, so a created agent appears and a deleted
 * one goes without anyone opening the Team tab. People still come from the
 * account list, which is fetched here the first time a picker needs it.
 */
export function useAssigneeDirectory(): AssigneeDirectoryInput {
  const agents = aos.stores.agent.useState((state) => state.items);
  const users = aos.stores.workspace.useState((state) => state.directory.users);
  const self = aos.stores.auth.useState((state) => state.user);

  useEffect(() => {
    if (users.length > 0 || peopleRequested) return;
    peopleRequested = true;
    void aos.stores.workspace.actions.refreshDirectory().catch(() => {
      // The pickers still list the signed-in person and every agent, and the
      // next one to mount asks again.
      peopleRequested = false;
    });
  }, [users.length]);

  return useMemo(
    () => ({
      users,
      self,
      agents: agents.map(
        (agent): WorkspaceDirectoryAgent => ({
          id: agent.id,
          name: agent.name || agent.id,
          image: agent.image,
          role: agent.role,
          description: agent.description,
          orchestrator: Boolean(agent.orchestrator),
          processing: [],
        }),
      ),
    }),
    [agents, users, self],
  );
}
