import { AosStore } from "@/app/builders/store";
import { type FractalViewSummary } from "@/features/view/interfaces/view.interfaces";
import { api } from "@/lib/aos-facade";

export const ViewStore = AosStore.create("views")
  .withState({
    items: [] as FractalViewSummary[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (ctx) => {
    const response = await api.view.list.query({ query: {} });

    return {
      items: response.data?.views || [],
    };
  })
  .addAction("refresh", (ctx) => async () => {
    const response = await api.view.list.query({ query: {} });

    ctx.state.set({
      items: response.data?.views || [],
    });

    return {
      items: response.data?.views || [],
    };
  })
  .build();