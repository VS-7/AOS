import * as React from "react"
import { DotmSquare4 } from "@/components/ui/dotm-square-4"
import type { Agent } from "@/features/agent/interfaces/agent.interfaces"
import type { Chat } from "@/features/chat/interfaces/chat.interfaces"

interface ChatProcessingIndicatorProps {
  agents: Agent[]
  chat: Chat
}

/**
 * Ported from the original, which read live per-agent occupancy off a
 * WebSocket-fed store (igniter.stores.agent). AOS has no such store; the
 * same "who's working on this" fact is derived from the last Run recorded
 * on each message (see internal/domain/chat/entity.go's Run.Status), which
 * chats_get already returns and lib/realtime.ts already keeps fresh.
 */
export function ChatProcessingIndicator({ agents, chat }: ChatProcessingIndicatorProps) {
  const workingAgentIds = React.useMemo(() => {
    const ids = new Set<string>()
    for (const message of chat.messages) {
      for (const run of message.runs ?? []) {
        if (run.status === "pending" || run.status === "running") {
          ids.add(run.agentId)
        }
      }
    }
    return [...ids]
  }, [chat.messages])

  if (workingAgentIds.length === 0) {
    return null
  }

  const typingAgents = workingAgentIds.map(
    (id) => agents.find((agent) => agent.id === id)?.name ?? id,
  )

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
