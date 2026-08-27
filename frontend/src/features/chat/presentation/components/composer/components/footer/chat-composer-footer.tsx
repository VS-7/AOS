import { t } from "@/lib/i18n";
import * as React from "react"
import { Button } from "@/components/ui/button"
import { PromptInputFooter, PromptInputSubmit, PromptInputTools } from "@/components/ui/prompt-input"
import { cn } from "@/lib/utils"
import { HugeiconsIcon } from "@hugeicons/react"
import {
    ArrowUp01Icon,
  Loading01Icon,
  Mic02Icon,
  PlusSignIcon,
  SentIcon,
  SquareIcon,
} from "@hugeicons/core-free-icons"

interface ChatComposerFooterProps {
  disabled: boolean
  isProcessing: boolean
  isRecording: boolean
  isSending: boolean
  isStoppingChat: boolean
  onOpenCommand: () => void
  onStop: () => void
  onToggleRecording: () => void
}

export function ChatComposerFooter({
  disabled,
  isProcessing,
  isRecording,
  isSending,
  isStoppingChat,
  onOpenCommand,
  onStop,
  onToggleRecording,
}: ChatComposerFooterProps) {
  const isGenerating = isProcessing || isSending
  const submitStatus = isProcessing
    ? "streaming"
    : isSending
      ? "submitted"
      : "ready"

  const toolButtonClass =
    "size-8 rounded-md"

  return (
    <PromptInputFooter className="items-center gap-3 px-3 py-2">
      <PromptInputTools className="flex flex-1 items-center gap-2">
        <Button
          aria-label={t("Add files, folders, skills, instructions, or mentions")}
          className={toolButtonClass}
          onClick={onOpenCommand}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <HugeiconsIcon icon={PlusSignIcon} className="size-5" />
        </Button>
      </PromptInputTools>

      <div className="flex items-center gap-2">
        <Button
          aria-label={isRecording ? "Stop recording voice note" : "Record a voice note"}
          className={cn(
            toolButtonClass,
            isRecording &&
              "bg-destructive/10 text-destructive hover:bg-destructive/15 hover:text-destructive",
          )}
          onClick={onToggleRecording}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          {isRecording ? (
            <HugeiconsIcon icon={Loading01Icon} className="size-5 animate-spin" />
          ) : (
            <HugeiconsIcon icon={Mic02Icon} className="size-5" />
          )}
        </Button>

        <PromptInputSubmit
          className="size-8 rounded-md shadow-none"
          disabled={isGenerating ? isStoppingChat : disabled}
          onStop={isGenerating ? onStop : undefined}
          status={submitStatus}
          variant="default"
        >
          {isStoppingChat ? (
            <HugeiconsIcon icon={Loading01Icon} className="size-4 animate-spin" />
          ) : isProcessing ? (
            <HugeiconsIcon icon={SquareIcon} className="size-4" />
          ) : isSending ? (
            <HugeiconsIcon icon={Loading01Icon} className="size-4 animate-spin" />
          ) : (
            <HugeiconsIcon icon={ArrowUp01Icon} className="size-4" />
          )}
        </PromptInputSubmit>
      </div>
    </PromptInputFooter>
  )
}
