import { AosStore } from "@/app/builders/store";
import { type Agent } from "@/features/agent/interfaces/agent.interfaces";
import { api } from "@/lib/aos-facade";

/**
 * Agent store — roster + live occupancy.
 *
 * B3 of the final review: this used to source the roster from
 * `loadWorkspaceDirectory` → `workspace.directory`, `null` in
 * `command-map.ts` (Go has no combined users+agents endpoint) — an earlier
 * ruling (R25) marked `WorkspaceAgentsSection` dormant to paper over that,
 * which this final review reversed: `agents_list` is real and live, and
 * `null` is supposed to mean "Go doesn't have this", not "wire it up
 * later". Reads straight off `agents_list` (`command-map.ts`'s
 * `agent.list`) now.
 *
 * `occupancy` (chatId -> agentIds currently answering it) is seeded from
 * `chats_list`'s own `active` and kept fresh by
 * `chat:start-processing`/`chat:end-processing`.
 *
 * The seed is the half that was missing. Occupancy used to start empty on
 * every load and be filled only by realtime, so reloading the window during
 * a turn showed an idle agent, and a window opened while a turn was already
 * running never showed it at all — the events it needed had been delivered
 * before it existed. The daemon now records a run the moment a turn begins
 * (chat.Service.MarkRun) and completes that same run when it ends, so "who
 * is working" is a fact on the record rather than only an event in flight.
 */
export const AgentStore = AosStore.create("agents")
  .withState({
    items: [] as Agent[],
    occupancy: {} as Record<string, string[]>, // chatId -> agentIds
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async () => {
    const [items, occupancy] = await Promise.all([fetchAgents(), fetchOccupancy()]);
    return { items: items ?? [], occupancy };
  })
  .addAction(
    "setProcessing",
    (ctx) => (chatId: string, agentId: string, isProcessing: boolean) => {
      ctx.state.set((state) => {
        const current = state.occupancy[chatId] || [];
        const next = isProcessing
          ? [...new Set([...current, agentId])]
          : current.filter((id) => id !== agentId);

        return {
          ...state,
          occupancy: {
            ...state.occupancy,
            [chatId]: next,
          },
        };
      });
    },
  )
  .addAction("refresh", (ctx) => async () => {
    // A roster that could not be read is not an empty roster. Replacing the
    // agents with [] on a failed read made every screen that follows the
    // store — the agent editor included — act as if they had been deleted.
    const items = (await fetchAgents()) ?? ctx.state.get().items;
    // Occupancy is deliberately not re-read here. A refresh runs on any
    // record change — an agent created, a project renamed — and the server's
    // answer is a snapshot taken before whatever realtime has delivered
    // since, so re-reading it would overwrite live state with a staler
    // version of itself. The seed happens once, at preload; events keep it
    // current after that.
    const next = { items, occupancy: ctx.state.get().occupancy };
    ctx.state.set(next);
    return next;
  })
  .build();

/**
 * Who is working right now, by conversation.
 *
 * `chats_list` answers this beside the roster (`ListOutput.Active`), computed
 * from the runs on each transcript before the messages are stripped — a run
 * with no completion is a turn in flight. Asking the list rather than every
 * conversation is the point: one call answers it for all of them.
 *
 * A failure here is not worth a broken store: the indicator degrades to what
 * realtime delivers, which is what it was before this existed.
 */
async function fetchOccupancy(): Promise<Record<string, string[]>> {
  const response = await api.chat!.list!.query<{ active?: Record<string, string[]> }>();
  if (response.error) {
    console.error("[AgentStore] chat.list failed while seeding occupancy", response.error);
    return {};
  }
  return response.data?.active ?? {};
}

/**
 * The roster, as `agents_list` answers it — every field it carries.
 *
 * This used to keep six of them (id, name, image, role, description,
 * orchestrator). Everything else a screen asked of an agent was therefore
 * always undefined: searching by provider or model never matched, grouping by
 * skill put every agent under "General", and the editor could not tell a
 * changed record from an unchanged one because `updatedAt` was dropped too.
 *
 * `undefined` when the roster could not be read, so a caller can keep what it
 * has instead of mistaking a failure for "no agents".
 */
async function fetchAgents(): Promise<Agent[] | undefined> {
  const response = await api.agent!.list!.query<{ agents?: Array<Record<string, unknown>> }>();
  if (response.error) {
    console.error("[AgentStore] agent.list failed", response.error);
    return undefined;
  }
  const agents = response.data?.agents ?? [];

  return agents.map(
    (agent) =>
      ({
        ...agent,
        orchestrator: Boolean(agent["orchestrator"]),
      }) as Agent,
  );
}
