import * as React from "react"
import { aos } from "@/app/aos"
import { useRealtime } from "@/hooks/use-realtime"
import { DotmSquare4 } from "@/components/ui/dotm-square-4"
import { t } from "@/lib/i18n"

interface ChatProcessingIndicatorProps {
  chatId: string
}

export function ChatProcessingIndicator({ chatId }: ChatProcessingIndicatorProps) {
  const agents = aos.stores.agent.useState((s) => s.items)
  const occupancy = aos.stores.agent.useState((s) => s.occupancy[chatId] ?? [])

  if (occupancy.length === 0) {
    return null
  }

  const typingAgents = occupancy
    .map((id) => agents.find((agent) => agent.id === id)?.name)
    .filter(Boolean)

  if (typingAgents.length === 0) {
    return null
  }

  // No trailing "…".
  //
  // The animated mark to the left already says the work is in progress, and
  // the ellipsis said it a second time — in a string that wraps. On a narrow
  // panel the dots fell to their own line and read as a row of stray points
  // under the sentence.
  const label = typingAgents.length === 1
    ? t("{{agent}} is working", { agent: typingAgents[0] as string })
    : typingAgents.length === 2
      ? t("{{first}} and {{second}} are working", {
          first: typingAgents[0] as string,
          second: typingAgents[1] as string,
        })
      : t("{{agent}} and {{count}} others are working", {
          agent: typingAgents[0] as string,
          count: typingAgents.length - 1,
        })

  return (
    <div className="absolute -top-10 left-6 flex max-w-[calc(100%-3rem)] items-center gap-2 text-xs font-medium text-muted-foreground animate-in fade-in slide-in-from-bottom-2 duration-300 border px-3 py-1.5 rounded-sm bg-card">
      {/* The running mark: it is what says "still going", so it never shrinks
          away and never wraps below the sentence. */}
      <DotmSquare4 className="size-4 shrink-0" />
      <span className="truncate whitespace-nowrap">{label}</span>
    </div>
  )
}
