import { AtSign } from "lucide-react"
import { aos } from "@/app/aos"
import { cn } from "@/lib/utils"

interface ChatInlineMentionTagProps {
  className?: string
  id: string
}

export function ChatInlineMentionTag({ className, id }: ChatInlineMentionTagProps) {
  return (
    <button
      className={cn(
        "inline-flex h-7 max-w-full items-center gap-1.5 rounded-full border border-emerald-200/80 bg-emerald-50 px-2.5 text-[12px] font-medium text-emerald-700 hover:bg-emerald-100 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200",
        className,
      )}
      onClick={() => aos.stores.viewport.actions.openSettings("workspace.agents")}
      type="button"
    >
      <AtSign className="size-3.5 shrink-0" />
      <span className="truncate">{id}</span>
    </button>
  )
}
