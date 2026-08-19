import { AosStore } from "@/app/builders/store";
import { type FractalRoutine } from "@/features/routine/interfaces/routine.interfaces";
import { api } from "@/lib/aos-facade";

export const RoutineStore = AosStore.create("routines")
  .withState({
    items: [] as FractalRoutine[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async () => {
    const response = await api.routine.list.query({ query: {} });

    return {
      items: response.data?.routines || [],
    };
  })
  .build();
