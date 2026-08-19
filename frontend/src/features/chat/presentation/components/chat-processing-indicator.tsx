import * as React from "react"
import { aos } from "@/app/aos"
import { useRealtime } from "@/hooks/use-realtime"
import { DotmSquare4 } from "@/components/ui/dotm-square-4"

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

  const label = typingAgents.length === 1
    ? `${typingAgents[0]} is working...`
    : typingAgents.length === 2
      ? `${typingAgents[0]} and ${typingAgents[1]} are working...`
      : `${typingAgents[0]} and ${typingAgents.length - 1} others are working...`

  return (
    <div className="absolute -top-10 left-6 flex items-center gap-2 text-xs font-medium text-muted-foreground animate-in fade-in slide-in-from-bottom-2 duration-300 border px-3 py-1.5 rounded-sm bg-card">
      <DotmSquare4 className="size-4" />
      <span>{label}</span>
    </div>
  )
}
