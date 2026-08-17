import * as React from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { client } from "@/lib/client";
import { usePromptInputController } from "@/components/ui/prompt-input";
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper";
import type {
  ChatComposerProps,
  ChatComposerSubmitMessage,
  ComposerMentionTarget,
} from "../composer.types";
import type { ChatComposerRichTextInputRef } from "../components/body/chat-composer-rich-text-input";

interface UseChatComposerParams extends ChatComposerProps {
  commandRef: React.RefObject<HTMLDivElement | null>;
  editorRef: React.RefObject<ChatComposerRichTextInputRef | null>;
}

/**
 * Ported from the original's useChatComposer, with the data layer replaced:
 * Igniter's chat.send/chat.update/chat.stop mutations and file/skill/
 * instruction queries become one client.invoke("chats_send", ...) call —
 * that command group is only list/get/create/send (see
 * internal/domain/chat/commands.go), so there is no context to clear and no
 * run to stop from here, and no file/skill/instruction picker to back.
 *
 * "isProcessing" no longer reads a live WebSocket occupancy store (AOS has
 * none); it's derived from the last Run recorded on each message, which
 * chats_get already returns and lib/realtime.ts already keeps fresh on
 * chat.done.
 */
export function useChatComposer({
  agents,
  chat,
  isDirectMessage = false,
  onSent,
  commandRef,
  editorRef,
}: UseChatComposerParams) {
  const controller = usePromptInputController();
  const queryClient = useQueryClient();
  const latestSyncedValueRef = React.useRef<string>("");

  const [commandOpen, setCommandOpen] = React.useState(false);
  const [commandQuery, setCommandQuery] = React.useState("");
  const [mentionState, setMentionState] =
    React.useState<ReturnType<typeof ComposerHelper.getActiveMention>>(null);

  const { mutate: sendMessage, isPending: isSending } = useMutation({
    mutationFn: async (text: string) =>
      client.invoke("chats_send", {
        chat: chat.id,
        text,
        _reasoning: "the person submitted the chat composer",
      }),
    onSuccess: () => {
      setMentionState(null);
      setCommandOpen(false);
      setCommandQuery("");
      latestSyncedValueRef.current = "";
      void queryClient.invalidateQueries({ queryKey: ["chat", chat.id] });
      onSent?.();
    },
    onError: (error: unknown) => {
      toast.error(error instanceof Error ? error.message : "Unable to send message.");
    },
  });

  const processingAgentIds = React.useMemo(() => {
    const ids = new Set<string>();
    for (const message of chat.messages) {
      for (const run of message.runs ?? []) {
        if (run.status === "pending" || run.status === "running") {
          ids.add(run.agentId);
        }
      }
    }
    return [...ids];
  }, [chat.messages]);

  const isProcessing = processingAgentIds.length > 0;
  const isBusy = isSending;
  const hasContent = latestSyncedValueRef.current.trim().length > 0;

  const closeCommand = React.useCallback(() => {
    setCommandOpen(false);
    setCommandQuery("");
  }, []);

  const commandMentionTargets = React.useMemo(() => {
    return ComposerHelper.resolveMentionTargets({
      agents,
      chat,
      query: mentionState?.query ?? commandQuery.trim(),
    });
  }, [agents, chat, commandQuery, mentionState?.query]);

  const syncMentionState = React.useCallback(
    (nextValue: string, caret = nextValue.length) => {
      latestSyncedValueRef.current = nextValue;

      if (nextValue !== controller.textInput.value) {
        controller.textInput.setInput(nextValue);
      }

      const nextMention = ComposerHelper.getActiveMention(nextValue, caret);
      setMentionState(nextMention);

      if (nextMention) {
        setCommandQuery(nextMention.query);
        setCommandOpen(true);
        return;
      }

      setCommandQuery("");
      setCommandOpen(false);
    },
    [controller.textInput],
  );

  React.useEffect(() => {
    if (!commandOpen) {
      return;
    }

    const frame = requestAnimationFrame(() => {
      const input = commandRef.current?.querySelector(
        "input",
      ) as HTMLInputElement | null;
      input?.focus();
      input?.select();
    });

    return () => cancelAnimationFrame(frame);
  }, [commandOpen, commandRef]);

  const handleMentionSelect = React.useCallback(
    (target: ComposerMentionTarget) => {
      editorRef.current?.insertMention(target.mentionId);
      setMentionState(null);
      closeCommand();
    },
    [closeCommand, editorRef],
  );

  const handleSubmit = React.useCallback(
    (submitMessage: ChatComposerSubmitMessage) => {
      if (isBusy) {
        return;
      }

      const text = submitMessage.text.trim();
      if (!text) {
        return;
      }

      sendMessage(text);

      controller.textInput.clear();
      editorRef.current?.clear();
      latestSyncedValueRef.current = "";
    },
    [controller.textInput, editorRef, isBusy, sendMessage],
  );

  return {
    closeCommand,
    commandMentionTargets,
    commandOpen,
    commandQuery,
    controller,
    handleMentionSelect,
    handleSubmit,
    hasContent,
    isBusy,
    isDirectMessage,
    isProcessing,
    isSending,
    mentionState,
    openCommand: () => {
      setMentionState(null);
      setCommandOpen(true);
    },
    setCommandOpen,
    setCommandQuery,
    syncMentionState,
  };
}
