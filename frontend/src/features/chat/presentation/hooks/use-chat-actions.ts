import * as React from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { aos } from "@/app/aos";
import { useAlert } from "@/components/ui/alert-provider";
import { errorMessage } from "@/lib/aos-facade";
import { t } from "@/lib/i18n";
import { closeChatTab } from "../helpers/open-chat-tab.helper";

/** The part of a chat the actions need. */
export interface ChatActionsTarget {
  id: string;
  title?: string;
  kind?: string;
}

export interface ChatActionsOptions {
  /** After a rename the daemon accepted. */
  onRenamed?: () => void;
  /**
   * After the transcript was emptied. The live transcript only ever merges
   * messages in, so the caller has to be told to take the server's version
   * whole — see `useChat`'s `replaceAndRefresh`.
   */
  onCleared?: () => void;
  /** After the conversation was deleted. */
  onDeleted?: () => void;
}

/**
 * Drops every trace of a deleted conversation from this window.
 *
 * The tab closes whether or not it is the focused one: a background tab kept
 * the deleted transcript and a composer that wrote into nothing. Its cached
 * read goes too, so nothing refetches an id the workspace no longer has — the
 * realtime invalidation of every chat read did exactly that, on every chat
 * write, and each refetch was another "no conversation" refusal.
 */
export function forgetChat(
  queryClient: Pick<QueryClient, "removeQueries">,
  chatId: string,
): void {
  closeChatTab(chatId);
  queryClient.removeQueries({ queryKey: ["chat", "getById", { chat: chatId }] });
  void aos.stores.chat.actions.refresh();
}

/**
 * Rename, clear and delete for any conversation.
 *
 * These lived inside the sidebar's channel row, so a DM, a task thread or a
 * run transcript could not be renamed or deleted anywhere — including a
 * conversation somebody opened with themselves by mistake. Clearing had no
 * confirmation at all: `/clear` and Enter threw the whole transcript away.
 * Both destructive actions now ask first, the same way.
 */
export function useChatActions(
  chat: ChatActionsTarget,
  options: ChatActionsOptions = {},
) {
  const { confirm } = useAlert();
  const queryClient = useQueryClient();
  const isChannel = (chat.kind ?? "channel") === "channel";
  const optionsRef = React.useRef(options);
  optionsRef.current = options;

  const { mutate: update, loading: isRenaming } = aos.client.chat.update.useMutation({
    onSuccess: () => {
      void aos.stores.chat.actions.refresh();
      optionsRef.current.onRenamed?.();
      toast.success(isChannel ? t("Channel renamed.") : t("Conversation renamed."));
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error) ?? t("Unable to rename this conversation."));
    },
  });

  const { mutate: destroy, loading: isDeleting } = aos.client.chat.delete.useMutation({
    onSuccess: () => {
      forgetChat(queryClient, chat.id);
      optionsRef.current.onDeleted?.();
      toast.success(isChannel ? t("Channel deleted.") : t("Conversation deleted."));
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error) ?? t("Unable to delete this conversation."));
    },
  });

  const { mutate: empty, loading: isClearing } = aos.client.chat.clear.useMutation({
    onSuccess: () => {
      optionsRef.current.onCleared?.();
      toast.success(t("Chat context cleared."));
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error) ?? t("Unable to clear chat context."));
    },
  });

  const title = chat.title?.trim() || t("this conversation");

  const rename = React.useCallback(
    (next: string) => {
      const trimmed = next.trim();
      if (!trimmed || trimmed === chat.title?.trim()) return false;
      update({ params: { chat: chat.id }, body: { title: trimmed } });
      return true;
    },
    [chat.id, chat.title, update],
  );

  const remove = React.useCallback(async () => {
    // The application's own dialog, not `window.confirm`: WKWebView routes
    // `confirm()` to a delegate Wails does not implement, which answers false
    // without drawing anything. See lib/wails.ts.
    const accepted = await confirm({
      title: t('Delete "{{title}}"?', { title }),
      description: isChannel
        ? t("This channel and its messages cannot be recovered.")
        : t("This conversation and its messages cannot be recovered."),
      confirmText: t("Delete"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!accepted) return;
    // The tab goes first, before the delete is even sent. The daemon announces
    // the write on the realtime channel before it answers the call, and that
    // announcement refetches every open chat — so a tab still mounted at that
    // moment asked for the conversation being deleted and was refused. If the
    // delete is refused instead, the conversation is still in the sidebar to
    // reopen, and the toast says why.
    closeChatTab(chat.id);
    destroy({ params: { chat: chat.id } });
  }, [chat.id, confirm, destroy, isChannel, title]);

  const clear = React.useCallback(async () => {
    const accepted = await confirm({
      title: t('Clear "{{title}}"?', { title }),
      description: t(
        "Every message in this conversation is deleted for everyone in it. The conversation itself stays. This cannot be undone.",
      ),
      confirmText: t("Clear"),
      cancelText: t("Cancel"),
      variant: "destructive",
    });
    if (!accepted) return;
    empty({ params: { chat: chat.id } });
  }, [chat.id, confirm, empty, title]);

  return { rename, remove, clear, isRenaming, isDeleting, isClearing };
}
