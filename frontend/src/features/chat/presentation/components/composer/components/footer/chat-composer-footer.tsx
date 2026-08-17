import { Button } from "@/components/ui/button"
import { PromptInputFooter, PromptInputSubmit, PromptInputTools } from "@/components/ui/prompt-input"
import { HugeiconsIcon } from "@hugeicons/react"
import { ArrowUp01Icon, Loading01Icon, PlusSignIcon } from "@hugeicons/core-free-icons"

/**
 * Trimmed from the original: no mic button (audio recording had nowhere to
 * send its clip — see the command menu's comment) and no stop button
 * (chats_send has no cancel counterpart; AOS's chats group is only
 * list/get/create/send, see internal/domain/chat/commands.go).
 */
interface ChatComposerFooterProps {
  disabled: boolean
  isProcessing: boolean
  isSending: boolean
  onOpenCommand: () => void
}

export function ChatComposerFooter({
  disabled,
  isProcessing,
  isSending,
  onOpenCommand,
}: ChatComposerFooterProps) {
  const isBusy = isProcessing || isSending
  const submitStatus = isProcessing ? "streaming" : isSending ? "submitted" : "ready"

  return (
    <PromptInputFooter className="items-center gap-3 px-3 py-2">
      <PromptInputTools className="flex flex-1 items-center gap-2">
        <Button
          aria-label="Mention an agent"
          className="size-8 rounded-md"
          onClick={onOpenCommand}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <HugeiconsIcon icon={PlusSignIcon} className="size-5" />
        </Button>
      </PromptInputTools>

      <div className="flex items-center gap-2">
        <PromptInputSubmit
          className="size-8 rounded-md shadow-none"
          disabled={isBusy || disabled}
          status={submitStatus}
          variant="default"
        >
          {isBusy ? (
            <HugeiconsIcon icon={Loading01Icon} className="size-4 animate-spin" />
          ) : (
            <HugeiconsIcon icon={ArrowUp01Icon} className="size-4" />
          )}
        </PromptInputSubmit>
      </div>
    </PromptInputFooter>
  )
}
