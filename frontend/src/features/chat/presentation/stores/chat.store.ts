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
      items: (response.data?.chats ?? []) as Chat[],
    };
  })
  .addAction('refresh', (ctx) => async () => {
    // Same call as the preload, for the same reason.
    const response = await api.chat.list.query({});
    // A list that could not be read is not an empty list, and it is certainly
    // not `undefined`: storing the missing field made every sidebar tab call
    // `.filter` on nothing while rendering, and the error boundary took the
    // whole window down with it — on a transient daemon error, or a restart
    // while the sidebar remounted. What was on screen stays until a read
    // succeeds.
    if (response.error) {
      return { items: ctx.state.get().items };
    }
    const items = (response.data?.chats ?? []) as Chat[];
    ctx.state.set({ items });
    return { items };
  })
  .build();
