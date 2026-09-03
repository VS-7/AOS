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
    // No `orderBy`: it named a choice that does not exist. `chats_list`
    // answers newest-updated first and offers no other ordering, so the field
    // was dropped by the decoder and the parameter only looked like a
    // setting somebody could change.
    const response = await api.chat.list.query({});

    return {
      items: response.data?.chats || [],
    };
  })
  .addAction('refresh', (ctx) => async () => {
    // Same call as the preload, for the same reason.
    const response = await api.chat.list.query({});
    ctx.state.set({ items: response.data?.chats })
    return { items: response.data?.chats }
  })
  .build();
