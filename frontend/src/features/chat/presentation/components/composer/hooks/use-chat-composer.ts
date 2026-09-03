import * as React from "react";
import { generateId } from "ai";
import { toast } from "sonner";
import { aos } from "@/app/aos";
import {
  usePromptInputAttachments,
  usePromptInputController,
} from "@/components/ui/prompt-input";
import type { WorkspaceFile } from "@/features/file/interfaces/file.interfaces";
import type { Instruction } from "@/features/instruction/interfaces/instruction.interfaces";
import type { Skill } from "@/features/skill/interfaces/skill.interfaces";
import type { ChatMessage } from "@/features/chat/interfaces/chat.interfaces";
import { ComposerHelper } from "@/features/chat/presentation/helpers/composer.helper";
import { ChatInlineMarkupHelper } from "@/features/chat/presentation/helpers/chat-inline-markup.helper";
import type {
  ChatComposerProps,
  ComposerMentionTarget,
  ComposerReference,
  ChatComposerSubmitMessage,
} from "../composer.types";
import type { ChatComposerRichTextInputRef } from "../components/body/chat-composer-rich-text-input";
import {
  CHAT_COMPOSER_SLASH_COMMANDS,
  type ChatComposerSlashCommandItem,
} from "../components/command/chat-composer-skills-command-menu";
import { useChatComposerAudio } from "./use-chat-composer-audio";
import { t } from "@/lib/i18n";

interface UseChatComposerParams extends ChatComposerProps {
  commandRef: React.RefObject<HTMLDivElement | null>;
  editorRef: React.RefObject<ChatComposerRichTextInputRef | null>;
}

export function useChatComposer({
  agents,
  chat,
  isDirectMessage = false,
  onCleared,
  onConfirmed,
  onFailed,
  onSent,
  userId,
  commandRef,
  editorRef,
}: UseChatComposerParams) {
  const controller = usePromptInputController();
  const attachments = usePromptInputAttachments();
  const audio = useChatComposerAudio();
  const pendingMessageIdRef = React.useRef<string | null>(null);
  const latestSyncedValueRef = React.useRef<string>("");

  const [commandOpen, setCommandOpen] = React.useState(false);
  const [commandQuery, setCommandQuery] = React.useState("");
  const [skillsOpen, setSkillsOpen] = React.useState(false);
  const [skillsQuery, setSkillsQuery] = React.useState("");
  const [mentionState, setMentionState] =
    React.useState<ReturnType<typeof ComposerHelper.getActiveMention>>(null);

  const deferredCommandQuery = React.useDeferredValue(commandQuery);
  const trimmedCommandQuery = deferredCommandQuery.trim();

  const filesSearchQuery = aos.client.file.search.useQuery({
    query: {
      includeIgnored: false,
      query: trimmedCommandQuery || "src",
      limit: 24,
    },
    enabled: commandOpen && trimmedCommandQuery.length > 0,
  });

  const rootFilesQuery = aos.client.file.list.useQuery({
    query: {
      includeIgnored: false,
      recursive: false,
    },
  });

  const skillsQueryRequest = aos.client.skill.list.useQuery({
    query: {
      query: "",
    },
  });

  const instructionsQuery = aos.client.instruction.list.useQuery();

  const { mutate: sendMessage, loading: isSending } =
    aos.client.chat.send.useMutation({
      onSuccess: (response) => {
        // `chats_send` (internal/domain/chat/service.go's Send) always
        // assigns its own message id — the client-generated one in
        // `nextMessage` below never round-trips — so the local echo has to
        // be swapped for this persisted message, not merged into it.
        const persisted = (
          response as { data?: { message?: ChatMessage } } | null | undefined
        )?.data?.message;

        if (pendingMessageIdRef.current && persisted) {
          onConfirmed?.(pendingMessageIdRef.current, persisted);
        }

        pendingMessageIdRef.current = null;
        setMentionState(null);
        setCommandOpen(false);
        setCommandQuery("");
        setSkillsOpen(false);
        setSkillsQuery("");
        latestSyncedValueRef.current = "";
      },
      onError: (error: any) => {
        if (pendingMessageIdRef.current) {
          onFailed?.(pendingMessageIdRef.current);
          pendingMessageIdRef.current = null;
        }

        toast.error(
          error?.error?.message || error?.message || "Unable to send message.",
        );
      },
    });

  // `chat.clear`, not `chat.update`: this used to send `{messages: []}` to the
  // update command, which has no such field — a rename that could drop a
  // transcript by carrying one field too many is a rename nobody can trust, so
  // Go keeps the two apart and so does this.
  const { mutate: clearChatContext, loading: isClearingContext } =
    aos.client.chat.clear.useMutation({
      onSuccess: () => {
        toast.success(t("Chat context cleared."));
        // The transcript is gone on the server; the live merge only adds, so
        // it has to be told to take the server's version whole. Without this
        // every cleared message stayed on screen.
        onCleared?.();
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message ||
            error?.message ||
            "Unable to clear chat context.",
        );
      },
    });

  const { mutate: stopChat, loading: isStoppingChat } =
    aos.client.chat.stop.useMutation({
      onSuccess: (result) => {
        // `onSuccess` receives the full `Envelope`, not the unwrapped
        // payload the original generated mutation hook handed its own
        // `onSuccess` — see `aos-facade.ts`'s `useMutation` doc comment.
        if (result?.data?.stopped) {
          toast.success(result.data.message ?? "Chat stopped.");
          return;
        }

        toast.info(result?.data?.message ?? "No active run was found to stop.");
      },
      onError: (error: any) => {
        toast.error(
          error?.error?.message || error?.message || "Unable to stop chat.",
        );
      },
    });

  const processingAgentIds = aos.stores.agent.useState(
    (state) => state.occupancy[chat.id] ?? [],
  );

  const hasRunningExecution = React.useMemo(
    () =>
      chat.messages.some((message) => {
        const execution =
          message.metadata?.type === "agent"
            ? message.metadata.execution
            : undefined;
        return (
          execution?.status === "pending" || execution?.status === "running"
        );
      }),
    [chat.messages],
  );

  const isProcessing =
    processingAgentIds.length > 0 || hasRunningExecution;

  const handleStop = React.useCallback(() => {
    if (isStoppingChat) {
      return;
    }

    void stopChat({
      params: { chat: chat.id },
      body: {},
    });
  }, [chat.id, isStoppingChat, stopChat]);

  const instructions = React.useMemo(
    () => (instructionsQuery.data?.instructions ?? []) as Instruction[],
    [instructionsQuery.data?.instructions],
  );

  const workspaceInstructionReferences = React.useMemo(
    () =>
      instructions
        .filter(ComposerHelper.isWorkspaceWideInstruction)
        .map(ComposerHelper.toReferenceFromInstruction),
    [instructions],
  );

  const inlineSources = React.useMemo(
    () => ChatInlineMarkupHelper.getSourceTokens(controller.textInput.value),
    [controller.textInput.value],
  );

  const autoAttachedInstructionReferences = React.useMemo(() => {
    const map = new Map<string, ComposerReference>();

    workspaceInstructionReferences.forEach((reference) => {
      map.set(reference.id, reference);
    });

    inlineSources
      .filter(
        (source) =>
          source.sourceType === "file" || source.sourceType === "folder",
      )
      .forEach((source) => {
        if (!source.path) {
          return;
        }

        ComposerHelper.getInstructionMatchesForPath(instructions, source.path)
          .map(ComposerHelper.toReferenceFromInstruction)
          .forEach((instructionReference) => {
            map.set(instructionReference.id, instructionReference);
          });
      });

    return [...map.values()];
  }, [inlineSources, instructions, workspaceInstructionReferences]);

  const rootFiles = React.useMemo(
    () => (rootFilesQuery.data?.files ?? []).slice(0, 12) as WorkspaceFile[],
    [rootFilesQuery.data?.files],
  );

  const selectableFiles = React.useMemo(() => {
    const references = (
      trimmedCommandQuery
        ? ((filesSearchQuery.data?.files ?? []) as WorkspaceFile[])
        : rootFiles
    ).map((file) => ComposerHelper.toReferenceFromFile(file as WorkspaceFile));

    const selectedPaths = new Set(
      inlineSources
        .filter(
          (source) =>
            source.sourceType === "file" || source.sourceType === "folder",
        )
        .map((source) => source.path),
    );

    return references.filter(
      (reference) => !selectedPaths.has(reference.targetPath ?? ""),
    );
  }, [
    filesSearchQuery.data?.files,
    inlineSources,
    rootFiles,
    trimmedCommandQuery,
  ]);

  const selectableSkills = React.useMemo(() => {
    const selectedSkillNames = new Set(
      inlineSources
        .filter((source) => source.sourceType === "skill")
        .map((source) => source.name.toLowerCase()),
    );

    return ((skillsQueryRequest.data?.skills ?? []) as Skill[])
      .map(ComposerHelper.toReferenceFromSkill)
      .filter(
        (reference) =>
          !selectedSkillNames.has(
            (reference.tagName ?? reference.label).toLowerCase(),
          ),
      );
  }, [inlineSources, skillsQueryRequest.data?.skills]);

  const directory = aos.stores.workspace.useState(
    (s) => s.directory ?? { users: [], agents: [] },
  );
  const directoryUsers = directory.users;

  // Agent store can race empty on first paint; Team list already falls back to
  // directory.agents — mirror that so #team/@ mentions stay populated.
  const mentionAgents = React.useMemo(() => {
    if (agents.length > 0) {
      return agents;
    }
    return directory.agents.map(
      (agent) =>
        ({
          id: agent.id,
          name: agent.name,
          image: agent.image,
          role: agent.role,
          description: agent.description,
          orchestrator: agent.orchestrator,
        }) as (typeof agents)[number],
    );
  }, [agents, directory.agents]);

  React.useEffect(() => {
    if (agents.length > 0 || directory.agents.length > 0) {
      return;
    }
    void aos.stores.workspace.actions.refreshDirectory();
    void aos.stores.agent.actions.refresh();
  }, [agents.length, directory.agents.length]);

  const commandMentionTargets = React.useMemo(() => {
    return ComposerHelper.resolveMentionTargets({
      agents: mentionAgents,
      chat,
      directoryUsers,
      selfUserId: userId,
      query: mentionState?.query ?? trimmedCommandQuery,
    });
  }, [
    mentionAgents,
    chat,
    directoryUsers,
    mentionState?.query,
    trimmedCommandQuery,
    userId,
  ]);

  const slashCommands = React.useMemo<ChatComposerSlashCommandItem[]>(() => {
    return [
      CHAT_COMPOSER_SLASH_COMMANDS.STOP,
      CHAT_COMPOSER_SLASH_COMMANDS.CLEAR,
    ];
  }, []);

  const isBusy = isSending || audio.isRecording;
  const hasContent =
    latestSyncedValueRef.current.trim().length > 0 ||
    attachments.files.length > 0;

  const focusEditor = React.useCallback(() => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        editorRef.current?.focus();
      });
    });
  }, [editorRef]);

  const closeCommand = React.useCallback(() => {
    setCommandOpen(false);
    setCommandQuery("");
  }, []);

  const closeSkillsCommand = React.useCallback(() => {
    setSkillsOpen(false);
    setSkillsQuery("");
  }, []);

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
        setSkillsOpen(false);
        setSkillsQuery("");
        return;
      }

      setCommandQuery("");

      const nextSkillTrigger = ComposerHelper.getActiveSkillTrigger(
        nextValue,
        caret,
      );

      if (nextSkillTrigger) {
        setSkillsQuery(nextSkillTrigger.query);
        setSkillsOpen(true);
        return;
      }

      setSkillsOpen(false);
      setSkillsQuery("");
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

  const handleReferenceSelect = React.useCallback(
    (reference: ComposerReference) => {
      editorRef.current?.insertReference(reference);
      setMentionState(null);
      closeCommand();
      closeSkillsCommand();
    },
    [closeCommand, closeSkillsCommand, editorRef],
  );

  const handleSlashCommandSelect = React.useCallback(
    (item: ChatComposerSlashCommandItem) => {
      if (item.disabled) {
        return;
      }

      if (item.id === "stop") {
        void stopChat({
          params: { chat: chat.id },
          body: {},
        });
        closeSkillsCommand();
        editorRef.current?.focus();
        return;
      }

      if (item.id === "clear") {
        const trigger = ComposerHelper.getActiveSkillTrigger(
          latestSyncedValueRef.current,
          latestSyncedValueRef.current.length,
        );

        // Drop the in-flight "/clear" token from the editor before we clear
        // the persisted chat context, so the user does not see the literal
        // command lingering in the composer after the mutation succeeds.
        if (trigger) {
          const nextValue = `${latestSyncedValueRef.current.slice(0, trigger.range.start)}${latestSyncedValueRef.current.slice(trigger.range.end)}`;
          latestSyncedValueRef.current = nextValue;
          controller.textInput.setInput(nextValue);
        }

        clearChatContext({
          params: { chat: chat.id },
        });

        closeSkillsCommand();
        editorRef.current?.focus();
        return;
      }

      const currentValue = latestSyncedValueRef.current;
      const currentCaret = currentValue.length;
      const trigger = ComposerHelper.getActiveSkillTrigger(
        currentValue,
        currentCaret,
      );

      if (!trigger) {
        closeSkillsCommand();
        return;
      }

      const insertion = `/${item.id} `;
      const nextValue = `${currentValue.slice(0, trigger.range.start)}${insertion}${currentValue.slice(trigger.range.end)}`;
      const nextCaret = trigger.range.start + insertion.length;

      latestSyncedValueRef.current = nextValue;
      controller.textInput.setInput(nextValue);
      editorRef.current?.focus();
      closeSkillsCommand();
      void nextCaret;
    },
    [
      chat.id,
      clearChatContext,
      closeSkillsCommand,
      controller.textInput,
      editorRef,
      stopChat,
    ],
  );

  const confirmPendingAudio = React.useCallback(() => {
    const audioFile = audio.buildPendingAudioFile();

    if (!audioFile) {
      return;
    }

    attachments.add([audioFile]);
    audio.closePendingAudio(false);
  }, [attachments, audio]);

  const handleSubmit = React.useCallback(
    (submitMessage: ChatComposerSubmitMessage) => {
      if (isBusy) {
        return;
      }

      const parts = ComposerHelper.buildMessageParts({
        autoAttachedInstructionReferences,
        submitMessage,
      });

      if (parts.length === 0) {
        return;
      }

      if (
        parts.length === 1 &&
        parts[0].type === "text" &&
        /^\/stop(?:@\w+)?$/i.test(parts[0].text.trim())
      ) {
        handleStop();
        controller.textInput.setInput("");
        latestSyncedValueRef.current = "";
        return;
      }

      const now = new Date();

      const nextMessage: ChatMessage = {
        id: generateId(),
        role: "user",
        parts,
        metadata: {
          createdAt: now.toISOString(),
          updatedAt: now.toISOString(),
          type: "user",
          data: {
            id: userId ?? "",
          },
        },
      };

      pendingMessageIdRef.current = nextMessage.id;
      onSent?.(nextMessage);

      // C5 of the final review ("honest empty state" policy, R26):
      // `command-map.ts`'s `chat.send` entry drops every `file` part —
      // Go's `SendInput` has no field for an attachment at all, disclosed
      // there. `onSent` above already rendered `nextMessage` locally
      // (attachment included), so without this the user sees their own
      // attachment appear to send successfully and only discovers it never
      // reached the agent when the reply makes no reference to it — or
      // never, if they don't think to check. A toast at the moment of
      // loss, not a silent one.
      if (parts.some((part) => part.type === "file")) {
        toast.warning(t("Attachments aren't sent to the agent yet — only your text was delivered."));
      }

      sendMessage({
        params: {
          chat: chat.id,
        },
        body: {
          id: chat.id,
          message: nextMessage,
        },
      });

      // Clear input and attachments immediately after sending
      controller.textInput.clear();
      editorRef.current?.clear();
      attachments.clear();
      latestSyncedValueRef.current = "";
    },
    [
      autoAttachedInstructionReferences,
      attachments,
      chat.id,
      controller.textInput,
      editorRef,
      isBusy,
      onSent,
      sendMessage,
      userId,
    ],
  );

  return {
    attachments,
    autoAttachedInstructionReferences,
    closeCommand,
    closeSkillsCommand,
    commandMentionTargets,
    commandOpen,
    commandQuery,
    confirmPendingAudio,
    controller,
    handleMentionSelect,
    handleReferenceSelect,
    handleSlashCommandSelect,
    handleStop,
    handleSubmit,
    hasContent,
    isBusy,
    isProcessing,
    isClearingContext,
    isDirectMessage,
    isSending,
    isStoppingChat,
    mentionState,
    openCommand: () => {
      setMentionState(null);
      setCommandOpen(true);
      setSkillsOpen(false);
      setSkillsQuery("");
    },
    pendingAudioClip: audio.pendingAudioClip,
    selectableFiles,
    selectableSkills,
    setCommandOpen,
    setCommandQuery,
    setSkillsOpen,
    setSkillsQuery,
    skillsOpen,
    skillsQuery,
    slashCommands,
    syncMentionState,
    toggleRecording: audio.handleRecordToggle,
    closePendingAudio: audio.closePendingAudio,
    isRecording: audio.isRecording,
    handleUploadSelect: () => {
      closeCommand();
      closeSkillsCommand();
      attachments.openFileDialog();
      focusEditor();
    },
  };
}
