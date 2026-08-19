import * as React from "react"
import { cn } from "@/lib/utils"
import { ChatInlineMarkupHelper } from "../../helpers/chat-inline-markup.helper"
import { ChatInlineMentionTag } from "./chat-inline-mention-tag"
import { ChatInlineSourceTag } from "./chat-inline-source-tag"

interface ChatInlineRichTextProps {
  className?: string
  inlineTagClassName?: string
  text: string
}

export function ChatInlineRichText({
  className,
  inlineTagClassName,
  text,
}: ChatInlineRichTextProps) {
  const tokens = React.useMemo(() => ChatInlineMarkupHelper.parse(text), [text])

  return (
    <span className={cn("whitespace-pre-wrap break-words", className)}>
      {tokens.map((token, index) => {
        if (token.type === "text") {
          return (
            <React.Fragment key={`text-${index}`}>
              {token.value}
            </React.Fragment>
          )
        }

        if (token.type === "mention") {
          return (
            <span
              className="align-middle"
              data-inline-markup={token.markup}
              key={`mention-${token.id}-${index}`}
            >
              <ChatInlineMentionTag className={inlineTagClassName} id={token.id} />
            </span>
          )
        }

        return (
          <span
            className="align-middle"
            data-inline-markup={token.markup}
            key={`source-${token.sourceType}-${token.name}-${index}`}
          >
            <ChatInlineSourceTag
              className={inlineTagClassName}
              name={token.name}
              path={token.path}
              sourceType={token.sourceType}
            />
          </span>
        )
      })}
    </span>
  )
}
