import { AosStore } from "@/app/builders/store";
import { type FractalAgent } from "@/features/agent/interfaces/agent.interfaces";
import { FractalChatKindHelper } from "@/features/chat/services/chat/chat-kind.helper";
import {
  invalidateWorkspaceDirectoryCache,
  loadWorkspaceDirectory,
} from "@/features/workspace/presentation/helpers/workspace-directory.fetch";

/**
 * Agent store — roster + live occupancy.
 *
 * Preload shares {@link loadWorkspaceDirectory} with the workspace store
 * directory field so boot does not duplicate HTTP. Realtime `setProcessing`
 * keeps occupancy fresh after the seed.
 */
export const AgentStore = AosStore.create("agents")
  .withState({
    items: [] as FractalAgent[],
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
    const directory = await loadWorkspaceDirectory("current");
    const processingByAgent = new Map(
      directory.agents.map((agent) => [agent.id, agent.processing] as const),
    );
    const occupancy =
      FractalChatKindHelper.occupancy_from_processing_index(processingByAgent as any);

    const items = directory.agents.map(
      (agent) =>
        ({
          id: agent.id,
          name: agent.name,
          image: agent.image,
          role: agent.role,
          description: agent.description,
          orchestrator: agent.orchestrator,
        }) as FractalAgent,
    );

    return {
      items,
      occupancy,
    };
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
    invalidateWorkspaceDirectoryCache();
    const directory = await loadWorkspaceDirectory("current", { force: true });
    const processingByAgent = new Map(
      directory.agents.map((agent) => [agent.id, agent.processing] as const),
    );
    const occupancy =
      FractalChatKindHelper.occupancy_from_processing_index(processingByAgent as any);

    const items = directory.agents.map(
      (agent) =>
        ({
          id: agent.id,
          name: agent.name,
          image: agent.image,
          role: agent.role,
          description: agent.description,
          orchestrator: agent.orchestrator,
        }) as FractalAgent,
    );

    const next = { items, occupancy };
    ctx.state.set(next);
    return next;
  })
  .build();
