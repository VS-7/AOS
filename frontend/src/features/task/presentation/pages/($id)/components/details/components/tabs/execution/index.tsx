import { PlayIcon } from "lucide-react";
import { t } from "@/lib/i18n";
import type { UseChatResult } from "@/features/chat/presentation/hooks/use-chat"
import { Task } from "@/features/task/interfaces/task.interfaces"
import { ChatTimeline } from "@/components/ui/chat-timeline"

export function TaskExecutionTab({
  task,
  liveChat,
}: {
  task: Task
  liveChat?: UseChatResult | null
}) {
  // A task that never ran has no conversation, and the tab used to render
  // nothing at all — an empty panel that looked like it was still loading.
  if (!task.chat) {
    return (
      <div className="flex flex-col items-center gap-2 px-4 py-10 text-center">
        <PlayIcon className="size-5 text-muted-foreground" />
        <p className="text-sm font-medium">{t("No execution yet")}</p>
        <p className="text-xs text-muted-foreground">
          {t("When an agent works on this task, its conversation and runs appear here.")}
        </p>
      </div>
    )
  }

  if (liveChat) {
    return (
      <ChatTimeline
        isLoading={liveChat.isLoading}
        messages={liveChat.messages}
        title={t("Execution Timeline")}
      />
    )
  }

  return <ChatTimeline chatId={task.chat} title={t("Execution Timeline")} />
}
