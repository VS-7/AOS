import * as React from "react"
import { PromptInputHeader } from "@/components/ui/prompt-input"
import { ChatComposerAttachmentsDisplay } from "../attachments/chat-composer-attachments-display"

/**
 * Composer header — only rendered while attachments are present.
 */
export function ChatComposerHeader() {
  return (
    <PromptInputHeader className="flex items-center gap-2 overflow-x-auto px-3 py-3">
      <ChatComposerAttachmentsDisplay />
    </PromptInputHeader>
  )
}
