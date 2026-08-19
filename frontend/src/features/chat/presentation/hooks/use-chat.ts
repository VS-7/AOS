import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "@/lib/client";
import type { Chat, Message } from "@/features/chat/interfaces/chat.interfaces";

export interface UseChatResult {
  chat: Chat | null;
  /**
   * Added for `task`'s execution timeline (`($id)/components/details/
   * components/tabs/execution/index.tsx`, `main/index.tsx`), which reads
   * `liveChat.messages` — a field the Fractal original's richer `useChat`
   * had (it tracked a live-streaming array separately) but this
   * intentionally-simplified port didn't need until now. `chat.messages`
   * is already fetched as part of the `chats_get` response (`Chat.messages:
   * Message[]`, `interfaces/chat.interfaces.ts`), so this is a derived
   * read, not a second fetch.
   */
  messages: Message[];
  isLoading: boolean;
  isRefreshing: boolean;
  refresh: () => void;
}

/**
 * Ported from the original's useChat, much simplified: that hook merges
 * partial realtime snapshots into local state itself, because Igniter's
 * `chat:refresh` payload can carry just one changed message. AOS's
 * lib/realtime.ts already invalidates the ["chat", chatId] query centrally
 * on `chat.done` (see App.tsx's useRealtime), so a plain refetch is the
 * whole job — TanStack Query owns the merge, this hook doesn't need to.
 */
export function useChat({ chatId, enabled = true }: { chatId: string; enabled?: boolean }): UseChatResult {
  const queryClient = useQueryClient();

  const chatQuery = useQuery({
    queryKey: ["chat", chatId],
    queryFn: async () =>
      (await client.invoke("chats_get", {
        chat: chatId,
        _reasoning: "the chat screen is open",
      })) as Chat,
    enabled: enabled && chatId !== "",
  });

  const refresh = React.useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["chat", chatId] });
  }, [queryClient, chatId]);

  return {
    chat: chatQuery.data ?? null,
    messages: chatQuery.data?.messages ?? [],
    isLoading: chatQuery.isLoading,
    isRefreshing: chatQuery.isFetching,
    refresh,
  };
}
