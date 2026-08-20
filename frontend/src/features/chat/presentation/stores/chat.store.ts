import { AosStore } from "@/app/builders/store";
import { api } from "@/lib/aos-facade";
import { Chat } from "../../interfaces/chat.interfaces";

export const ChatStore = AosStore.create("agents")
  .withState({
    items: [] as Chat[],
  })
  .withPersistence({
    enabled: false,
  })
  .withNamespace({
    resolver: ({ namespaces }) => namespaces.workspaceId,
    strategy: "memory-partition",
  })
  .withPreload(async (ctx) => {
    const response = await api.chat.list.query({
      query: { orderBy: "updatedAt" },
    });

    return {
      items: response.data?.chats || [],
    };
  })
  .addAction('refresh', (ctx) => async () => {
    const response = await api.chat.list.query({
      query: { orderBy: "updatedAt" },
    });
    ctx.state.set({ items: response.data?.chats })
    return { items: response.data?.chats }
  })
  .build();
