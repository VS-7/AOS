import { AosStore } from "@/app/builders/store";
import type { FractalProject } from "@/features/project/interfaces/project.interfaces";
import { api } from "@/lib/aos-facade";

export const ProjectStore = AosStore.create("projects")
  .withState({
    items: [] as FractalProject[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (_ctx) => {
    const response = await api.project.list.query({ query: {} });

    return {
      items: response.data?.projects || [],
    };
  })
  .addAction("refresh", (_ctx) => async () => {
    const response = await api.project.list.query({ query: {} });

    return {
      items: response.data?.projects || [],
    };
  })
  .build();
