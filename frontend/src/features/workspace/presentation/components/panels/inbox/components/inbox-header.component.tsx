import * as React from "react"
import { CheckCheck, Clock } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { aos } from "@/app/aos"

export function InboxHeader() {
  const unreadCount = aos.stores.activity.useState((s) => s.unreadCount)
  const { markAllAsRead } = aos.stores.activity.useActions()

  return (
    <div className="flex items-center gap-2 p-6 shrink-0">
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <Clock className="size-4" />

        <span className="text-sm">
          Activity
        </span>
        {unreadCount > 0 && (
          <span className="text-xs bg-primary text-primary-foreground rounded-md px-1.5 py-0.5 leading-none">
            {unreadCount}
          </span>
        )}
      </div>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button 
            variant="ghost" 
            size="icon" 
            className="h-7 w-7 text-muted-foreground" 
            onClick={() => markAllAsRead().then(
              () => toast.success("All activities marked as read"),
              (err) => toast.error(err?.message ?? "Failed to mark activities as read"),
            )}
            disabled={unreadCount === 0}
          >
            <CheckCheck className="size-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          Mark all as read
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
