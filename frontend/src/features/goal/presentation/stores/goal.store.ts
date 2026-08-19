import { AosStore } from "@/app/builders/store";

import { api } from "@/lib/aos-facade";
import type { FractalGoal } from "@/features/goal/interfaces/goal.interfaces";

export const GoalStore = AosStore.create("goals")
  .withState({
    items: [] as FractalGoal[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (_ctx) => {
    const response = await api.goal.list.query({ query: {} });

    return {
      items: response.data?.goals || [],
    };
  })
  .addAction("refresh", (_ctx) => async () => {
    const response = await api.goal.list.query({ query: {} });

    return {
      items: response.data?.goals || [],
    };
  })
  .build();
