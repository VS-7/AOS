import { t } from "@/lib/i18n";
import * as React from "react"
import { CheckCheck, Clock } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { aos } from "@/app/aos"
import { errorMessage } from "@/lib/aos-facade"

export function InboxHeader() {
  const unreadCount = aos.stores.activity.useState((s) => s.unreadCount)
  const { markAllAsRead } = aos.stores.activity.useActions()
  // A slow or frozen daemon used to leave this button enabled and unchanged
  // for as long as the call took, inviting a second and third click with
  // nothing to say the first one was still on its way.
  const [isMarking, setIsMarking] = React.useState(false)

  const handleMarkAll = React.useCallback(async () => {
    setIsMarking(true)
    try {
      await markAllAsRead()
      toast.success(t("All activities marked as read"))
    } catch (error) {
      toast.error(t("Failed to mark activities as read"), { description: errorMessage(error) })
    } finally {
      setIsMarking(false)
    }
  }, [markAllAsRead])

  return (
    <div className="flex items-center gap-2 p-6 shrink-0">
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <Clock className="size-4" />

        <span className="text-sm">
          {t("Activity")}
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
            onClick={() => void handleMarkAll()}
            disabled={unreadCount === 0 || isMarking}
            aria-busy={isMarking}
            aria-label={t("Mark all as read")}
          >
            {isMarking ? <Spinner /> : <CheckCheck className="size-4" />}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {t("Mark all as read")}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
