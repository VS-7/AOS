import * as React from "react"
import { Attachment, AttachmentInfo, AttachmentPreview, AttachmentRemove, Attachments } from "@/components/ui/attachments"
import { usePromptInputAttachments } from "@/components/ui/prompt-input"

export function ChatComposerAttachmentsDisplay() {
  const attachments = usePromptInputAttachments()

  if (attachments.files.length === 0) {
    return null
  }

  return (
    <Attachments variant="inline">
      {attachments.files.map((attachment) => (
        <Attachment
          data={attachment}
          key={attachment.id}
          onRemove={() => attachments.remove(attachment.id)}
        >
          <AttachmentPreview />
          <AttachmentInfo />
          <AttachmentRemove />
        </Attachment>
      ))}
    </Attachments>
  )
}
