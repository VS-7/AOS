import { AosStore } from "@/app/builders/store";
import type { FractalArtifactListItem } from "@/features/artifact/interfaces/artifact.interfaces";
import { api } from "@/lib/aos-facade";

export const ArtifactStore = AosStore.create("artifacts")
  .withState({
    items: [] as FractalArtifactListItem[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async () => {
    const response = await api.artifact.list.query({ query: {} });

    return {
      items: response.data?.artifacts || [],
    };
  })
  .addAction("refresh", (ctx) => async () => {
    const response = await api.artifact.list.query({ query: {} });

    ctx.state.set({
      items: response.data?.artifacts || [],
    });

    return {
      items: response.data?.artifacts || [],
    };
  })
  .build();
