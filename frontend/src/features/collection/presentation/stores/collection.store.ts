import { AosStore } from "@/app/builders/store";
import type { CollectionDefinition } from "@/features/collection/presentation/helpers/collection-fields.helper";
import { api } from "@/lib/aos-facade";

export const CollectionStore = AosStore.create("collections")
  .withState({
    items: [] as CollectionDefinition[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (ctx) => {
    const response = await api.collection.list.query({ query: {} });

    return {
      items: response.data?.collections || [],
    };
  })
  .addAction("refresh", (ctx) => async () => {
    const response = await api.collection.list.query({ query: {} });

    ctx.state.set({
      items: response.data?.collections || [],
    });

    return {
      items: response.data?.collections || [],
    };
  })
  .build();