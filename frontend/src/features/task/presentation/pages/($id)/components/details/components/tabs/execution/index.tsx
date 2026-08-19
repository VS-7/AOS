import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat"
import { FractalTask } from "@/features/task/interfaces/task.interfaces"
import { ChatTimeline } from "@/components/ui/chat-timeline"

export function TaskExecutionTab({
  task,
  liveChat,
}: {
  task: FractalTask
  liveChat?: UseChatResult | null
}) {
  if (!task.chat) return null

  if (liveChat) {
    return (
      <ChatTimeline
        isLoading={liveChat.isLoading}
        messages={liveChat.messages}
        title="Execution Timeline"
      />
    )
  }

  return <ChatTimeline chatId={task.chat} title="Execution Timeline" />
}
