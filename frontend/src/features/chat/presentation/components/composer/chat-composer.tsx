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
              accept="image/*,audio/*,video/*,application/pdf,text/plain,.md,.txt,.json,.csv,.ts,.tsx,.js,.jsx,.yml,.yaml"
            className="overflow-hidden rounded-md border bg-popover"
              globalDrop
              inputGroupClassName="flex-col gap-0 border-0 bg-transparent shadow-none"
              maxFiles={12}
              multiple
              onError={(error) => toast.error(error.message)}
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
                  placeholder={
                    props.isDirectMessage
                      ? "Message this agent, attach files, or record a voice note..."
                      : "Message this channel, attach context, or type / to insert a skill or @ to mention a teammate..."
                  }
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
                onToggleRecording={composer.toggleRecording}
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
            onUploadSelect={composer.handleUploadSelect}
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
