import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { client } from "@/lib/client";
import type { Chat } from "@/features/chat/interfaces/chat.interfaces";

export interface UseChatResult {
  chat: Chat | null;
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
    isLoading: chatQuery.isLoading,
    isRefreshing: chatQuery.isFetching,
    refresh,
  };
}
