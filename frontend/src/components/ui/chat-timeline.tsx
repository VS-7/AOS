import { useQuery } from "@tanstack/react-query"
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { client } from "@/lib/client"
import {
  BotIcon,
  FileTextIcon,
  WrenchIcon,
} from "lucide-react"
import type { Chat, ChatMessage } from "@/features/chat/interfaces/chat.interfaces"

/**
 * A stored message is finished by the time chats_get returns it — the
 * in-progress turn is the separate streaming answer (lib/realtime.ts). This
 * timeline shows what happened, not a live per-part status.
 *
 * Task 9 replaced `chat.interfaces.ts`'s `Message`/`Part` (which mirrored
 * AOS's Go entity directly) with the recovered AOS `ChatMessage`
 * (`UIMessage<ChatMessageMetadata>` from the `ai` SDK — see
 * `features/chat/interfaces/chat.interfaces.ts`'s doc comment) so the bulk
 * of the freshly-copied chat/agent/workspace presentation code — which
 * imports that exact type — compiles. This file is the one hand-adapted
 * casualty of that swap: it's the sole consumer task's execution tab reads
 * (`liveChat.messages`), so it must keep working, but the AI-SDK part
 * union's tool-call variant now carries the tool name *in* `part.type`
 * (`"tool-${name}"` or `"dynamic-tool"` + `.toolName`) instead of a
 * `.toolName` field on a `"tool-call"` part — reading a still-real field
 * through an `any` cast rather than exhaustively narrowing a union this
 * function only cares about loosely.
 */
type TimelineEvent = {
  id: string
  kind: "reasoning" | "tool" | "text"
  title: string
  detail: string
}

function toExecutionEvents(messages: ChatMessage[]): TimelineEvent[] {
  const events: TimelineEvent[] = []

  for (const message of messages) {
    if (message.role !== "assistant" && message.metadata?.type !== "agent") continue

    for (const [index, rawPart] of (message.parts ?? []).entries()) {
      const id = `${message.id}:${index}`
      // Loosely typed on purpose — see this file's top comment.
      const part = rawPart as any

      if (part.type === "reasoning") {
        const detail = (part.text as string | undefined)?.trim()
        if (!detail) continue
        events.push({ id, kind: "reasoning", title: "Reasoning", detail })
        continue
      }

      if (part.type === "text") {
        const detail = (part.text as string | undefined)?.trim()
        if (!detail) continue
        events.push({ id, kind: "text", title: "Response", detail })
        continue
      }

      const isToolPart = typeof part.type === "string" && (part.type.startsWith("tool-") || part.type === "dynamic-tool")
      if (isToolPart) {
        const toolName = part.toolName ?? (part.type as string).replace(/^tool-/, "") ?? "Tool"
        const hasOutput = part.state === "output-available" && part.output !== undefined
        const detail = hasOutput ? stringify(part.output) : `Tool call: ${toolName}`
        events.push({ id, kind: "tool", title: toolName, detail })
      }
    }
  }

  return events
}

function stringify(value: unknown): string {
  if (value === undefined || value === null) return ""
  if (typeof value === "string") return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function eventIcon(kind: TimelineEvent["kind"]) {
  if (kind === "reasoning") return <BotIcon className="size-3.5 text-muted-foreground" />
  if (kind === "tool") return <WrenchIcon className="size-3.5 text-muted-foreground" />
  return <FileTextIcon className="size-3.5 text-muted-foreground" />
}

export interface ChatTimelineProps {
  chatId?: string
  messages?: ChatMessage[]
  isLoading?: boolean
  title?: string
}

export function ChatTimeline({
  chatId,
  messages,
  isLoading: externalLoading,
  title = "Execution Timeline",
}: ChatTimelineProps) {
  if (messages) {
    return (
      <ChatTimelineView
        isLoading={externalLoading ?? false}
        messages={messages}
        title={title}
      />
    )
  }

  if (!chatId) {
    return null
  }

  return <ChatTimelineLive chatId={chatId} title={title} />
}

function ChatTimelineLive({
  chatId,
  title,
}: {
  chatId: string
  title: string
}) {
  // Same query key features/chat/presentation/hooks/use-chat.ts uses for the
  // same chat: one cache entry, invalidated centrally by lib/realtime.ts on
  // `chat.done` — no manual refetch wiring needed here.
  const { data, isLoading } = useQuery({
    queryKey: ["chat", chatId],
    queryFn: async () =>
      (await client.invoke("chats_get", {
        chat: chatId,
        _reasoning: "the execution timeline is open",
      })) as Chat,
  })

  return (
    <ChatTimelineView
      isLoading={isLoading}
      messages={data?.messages ?? []}
      title={title}
    />
  )
}

function ChatTimelineView({
  isLoading,
  messages,
  title,
}: {
  isLoading: boolean
  messages: ChatMessage[]
  title: string
}) {
  const events = toExecutionEvents(messages)

  return (
    <div className="flex flex-col">
      <div className="mb-2 flex items-center justify-between px-1">
        <p className="text-xs font-semibold tracking-wide text-foreground/90">{title}</p>
        <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
          <span>{events.length} events</span>
        </div>
      </div>

      {isLoading ? (
        <p className="text-xs text-muted-foreground">Loading execution...</p>
      ) : events.length === 0 ? (
        <p className="text-xs text-muted-foreground">No execution events yet.</p>
      ) : (
        <Accordion type="multiple" className="w-full max-w-none pr-1 pb-2">
          {events.map((event, index) => (
            <AccordionItem key={event.id} value={event.id}>
              <AccordionTrigger className="px-2.5">
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  {eventIcon(event.kind)}
                  <span className="text-[11px] text-muted-foreground">#{index + 1}</span>
                  <span className="truncate text-xs font-medium text-foreground/90">{event.title}</span>
                </div>
              </AccordionTrigger>

              <AccordionContent className="px-2.5 pb-2.5">
                <div className="max-w-full overflow-hidden rounded-md border border-border/40 bg-background/60 p-2">
                  <p className="max-w-full overflow-hidden whitespace-pre-wrap break-all text-xs leading-relaxed text-muted-foreground">
                    {event.detail}
                  </p>
                </div>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      )}
    </div>
  )
}
