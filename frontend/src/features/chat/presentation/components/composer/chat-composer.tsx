import * as React from "react";
import { Popover, PopoverAnchor } from "@/components/ui/popover";
import {
  PromptInput,
  PromptInputBody,
  PromptInputProvider,
} from "@/components/ui/prompt-input";
import { toast } from "sonner";
import type { ChatComposerProps } from "./composer.types";
import { useChatComposer } from "./hooks/use-chat-composer";
import { ChatComposerHeader } from "./components/header/chat-composer-header";
import { ChatComposerFooter } from "./components/footer/chat-composer-footer";
import { ChatComposerCommandMenu } from "./components/command/chat-composer-command-menu";
import { ChatComposerSkillsCommandMenu } from "./components/command/chat-composer-skills-command-menu";
import { ChatComposerAudioConfirmationDialog } from "./components/audio-confirmation/chat-composer-audio-confirmation-dialog";
import {
  ChatComposerRichTextInput,
  type ChatComposerRichTextInputRef,
} from "./components/body/chat-composer-rich-text-input";
import { ChatProcessingIndicator } from "../chat-processing-indicator";
import { t } from "@/lib/i18n";

/**
 * Whether a file attached to a message reaches the agent.
 *
 * It does not: `chats_send` takes text and nothing else (see `chat.send` in
 * lib/command-map.ts), so the composer used to accept uploads, drops and
 * voice notes, show the chip, and discard the file on send — warning only
 * after the fact. Until the daemon can carry one, the composer does not offer
 * what it cannot deliver: no upload entry, no microphone, and a drop or paste
 * is refused on the spot with the reason.
 */
const ATTACHMENTS_REACH_THE_AGENT = false;

/** What the empty composer invites, for the surface it is on. */
function composerPlaceholder({
  isDirectMessage,
  kind,
}: Pick<ChatComposerProps, "isDirectMessage" | "kind">): string {
  if (isDirectMessage) return t("Message this agent…");
  switch (kind) {
    case "task":
      return t("Message this task's thread, or type / for commands and @ to mention a teammate…");
    case "run":
      return t("Message this run, or type / for commands…");
    case "dm":
      return t("Message this conversation…");
    default:
      return t("Message this channel, or type / for commands and skills and @ to mention a teammate…");
  }
}

function ChatComposerSurface(props: ChatComposerProps) {
  const composerRef = React.useRef<HTMLDivElement | null>(null);
  const commandRef = React.useRef<HTMLDivElement | null>(null);
  const editorRef = React.useRef<ChatComposerRichTextInputRef | null>(null);

  const composer = useChatComposer({
    ...props,
    commandRef,
    editorRef,
  });

  const anyMenuOpen = composer.commandOpen || composer.skillsOpen;

  const hasAttachments = composer.attachments.files.length > 0;

  return (
    <div
      className="pointer-events-none absolute inset-x-0 bottom-6 z-20"
      ref={composerRef}
    >
      <ChatProcessingIndicator chatId={props.chat.id} />

      <Popover
        onOpenChange={(open) => {
          if (composer.skillsOpen) {
            composer.setSkillsOpen(open);
          } else {
            composer.setCommandOpen(open);
          }

          if (!open) {
            composer.syncMentionState(composer.controller.textInput.value);
          }
        }}
        open={anyMenuOpen}
      >
        <PopoverAnchor asChild>
          <div className="pointer-events-auto px-6">
            <PromptInput
              accept={
                ATTACHMENTS_REACH_THE_AGENT
                  ? "image/*,audio/*,video/*,application/pdf,text/plain,.md,.txt,.json,.csv,.ts,.tsx,.js,.jsx,.yml,.yaml"
                  : undefined
              }
              className="overflow-hidden rounded-md border bg-popover"
              globalDrop
              inputGroupClassName="flex-col gap-0 border-0 bg-transparent shadow-none"
              // Zero while attachments cannot be delivered: every way a file
              // arrives (drop, paste, the native drop) goes through the same
              // capacity check, and a refused one says why right then.
              maxFiles={ATTACHMENTS_REACH_THE_AGENT ? 12 : 0}
              multiple
              onError={(error) =>
                error.code === "max_files" && !ATTACHMENTS_REACH_THE_AGENT
                  ? // One toast, not one per open chat tab: every mounted
                    // composer hears the same drop, and a fixed id collapses
                    // their refusals into one.
                    toast.error(
                      t("Attachments can't be sent to an agent yet. Only text is delivered."),
                      { id: "chat-attachments-not-delivered" },
                    )
                  : toast.error(error.message)
              }
              onSubmit={composer.handleSubmit}
            >
              {hasAttachments ? <ChatComposerHeader /> : null}

              <PromptInputBody>
                <ChatComposerRichTextInput
                  className="border-0 bg-transparent shadow-none text-left"
                  disabled={composer.isBusy}
                  onEscape={
                    composer.commandOpen
                      ? composer.closeCommand
                      : composer.skillsOpen
                        ? composer.closeSkillsCommand
                        : undefined
                  }
                  onSelectionChange={composer.syncMentionState}
                  placeholder={composerPlaceholder(props)}
                  ref={editorRef}
                  value={composer.controller.textInput.value}
                />
              </PromptInputBody>

              <ChatComposerFooter
                disabled={!composer.hasContent || composer.isBusy}
                isProcessing={composer.isProcessing}
                isRecording={composer.isRecording}
                isSending={composer.isSending}
                isStoppingChat={composer.isStoppingChat}
                onOpenCommand={composer.openCommand}
                onStop={composer.handleStop}
                onToggleRecording={
                  ATTACHMENTS_REACH_THE_AGENT ? composer.toggleRecording : undefined
                }
              />
            </PromptInput>
          </div>
        </PopoverAnchor>

        {composer.skillsOpen ? (
          <ChatComposerSkillsCommandMenu
            commandRef={commandRef}
            onCommandSelect={composer.handleSlashCommandSelect}
            onQueryChange={composer.setSkillsQuery}
            onReferenceSelect={composer.handleReferenceSelect}
            query={composer.skillsQuery}
            slashCommands={composer.slashCommands}
            selectableSkills={composer.selectableSkills}
            trigger={null}
          />
        ) : (
          <ChatComposerCommandMenu
            commandMentionTargets={composer.commandMentionTargets}
            commandQuery={composer.commandQuery}
            commandRef={commandRef}
            isDirectMessage={Boolean(props.isDirectMessage)}
            mentionState={composer.mentionState}
            onMentionSelect={composer.handleMentionSelect}
            onQueryChange={composer.setCommandQuery}
            onReferenceSelect={composer.handleReferenceSelect}
            onUploadSelect={
              ATTACHMENTS_REACH_THE_AGENT ? composer.handleUploadSelect : undefined
            }
            selectableFiles={composer.selectableFiles}
          />
        )}
      </Popover>

      <ChatComposerAudioConfirmationDialog
        clip={composer.pendingAudioClip}
        onConfirm={composer.confirmPendingAudio}
        onOpenChange={composer.closePendingAudio}
      />
    </div>
  );
}

export function ChatComposer(props: ChatComposerProps) {
  return (
    <PromptInputProvider>
      <ChatComposerSurface {...props} />
    </PromptInputProvider>
  );
}
