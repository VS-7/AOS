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
 * `occupancy` (chatId -> agentIds currently answering it) has no Go
 * source: `agents_list`'s `Agent` (`internal/domain/agent/entity.go`) has
 * no `processing` field the way the original combined directory
 * endpoint did, and the realtime signals that used to keep it fresh
 * (`chat:start-processing`/`chat:end-processing`) have no daemon
 * counterpart either — both are explicit `null`s in
 * `lib/realtime-event-map.ts`, with their own comment on why. Occupancy
 * starts, and stays, empty rather than showing stale or fabricated data;
 * `setProcessing` is kept as a real action for the day either gap closes.
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
    const items = await fetchAgents();
    return { items, occupancy: {} };
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
    const items = await fetchAgents();
    // Occupancy isn't part of what `refresh` re-fetches — it has no server
    // source (see the store's own doc comment) — so a manual refresh
    // (e.g. after creating an agent) doesn't clobber whatever realtime has
    // accumulated into it.
    const next = { items, occupancy: ctx.state.get().occupancy };
    ctx.state.set(next);
    return next;
  })
  .build();

async function fetchAgents(): Promise<Agent[]> {
  const response = await api.agent!.list!.query<{ agents?: Array<Record<string, unknown>> }>();
  if (response.error) {
    console.error("[AgentStore] agent.list failed", response.error);
  }
  const agents = response.data?.agents ?? [];

  return agents.map(
    (agent) =>
      ({
        id: agent["id"],
        name: agent["name"],
        image: agent["image"],
        role: agent["role"],
        description: agent["description"],
        orchestrator: Boolean(agent["orchestrator"]),
      }) as Agent,
  );
}
