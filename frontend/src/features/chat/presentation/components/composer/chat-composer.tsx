import * as React from "react";
import { Popover, PopoverAnchor } from "@/components/ui/popover";
import {
  PromptInput,
  PromptInputBody,
  PromptInputProvider,
} from "@/components/ui/prompt-input";
import type { ChatComposerProps } from "./composer.types";
import { useChatComposer } from "./hooks/use-chat-composer";
import { ChatComposerFooter } from "./components/footer/chat-composer-footer";
import { ChatComposerCommandMenu } from "./components/command/chat-composer-command-menu";
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

  return (
    <div
      className="pointer-events-none absolute inset-x-0 bottom-6 z-20"
      ref={composerRef}
    >
      <ChatProcessingIndicator agents={props.agents} chat={props.chat} />

      <Popover
        onOpenChange={(open) => {
          composer.setCommandOpen(open);

          if (!open) {
            composer.syncMentionState(composer.controller.textInput.value);
          }
        }}
        open={composer.commandOpen}
      >
        <PopoverAnchor asChild>
          <div className="pointer-events-auto px-6">
            <PromptInput
              className="overflow-hidden rounded-md border bg-popover"
              inputGroupClassName="flex-col gap-0 border-0 bg-transparent shadow-none"
              onSubmit={composer.handleSubmit}
            >
              <PromptInputBody>
                <ChatComposerRichTextInput
                  className="border-0 bg-transparent shadow-none text-left"
                  disabled={composer.isBusy}
                  onEscape={composer.commandOpen ? composer.closeCommand : undefined}
                  onSelectionChange={composer.syncMentionState}
                  placeholder={
                    props.isDirectMessage
                      ? "Message this agent..."
                      : "Message this channel, or type @ to mention an agent..."
                  }
                  ref={editorRef}
                  value={composer.controller.textInput.value}
                />
              </PromptInputBody>

              <ChatComposerFooter
                disabled={!composer.hasContent || composer.isBusy}
                isProcessing={composer.isProcessing}
                isSending={composer.isSending}
                onOpenCommand={composer.openCommand}
              />
            </PromptInput>
          </div>
        </PopoverAnchor>

        <ChatComposerCommandMenu
          commandMentionTargets={composer.commandMentionTargets}
          commandQuery={composer.commandQuery}
          commandRef={commandRef}
          isDirectMessage={Boolean(props.isDirectMessage)}
          mentionState={composer.mentionState}
          onMentionSelect={composer.handleMentionSelect}
          onQueryChange={composer.setCommandQuery}
        />
      </Popover>
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
