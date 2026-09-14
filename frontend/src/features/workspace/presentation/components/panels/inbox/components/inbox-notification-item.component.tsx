import * as React from "react"
import { useNavigate } from "@tanstack/react-router"
import { toast } from "sonner"
import { cn } from "@/lib/utils"
import { aos } from "@/app/aos"
import { errorMessage } from "@/lib/aos-facade"
import { useTranslation } from "@/lib/i18n"
import type { ActivityEntry } from "@/features/activity/interfaces/activity.interfaces"
import {
  activityTarget,
  activityTimeLabel,
  activityTitle,
} from "@/features/activity/presentation/helpers/activity-presentation.helper"

interface InboxNotificationItemProps {
  notification: ActivityEntry
}

/**
 * One inbox line: what happened, when, and whether you have seen it.
 *
 * It is a button because opening it is the point — the record it is about,
 * marked read on the way. It used to be a plain div that did nothing when
 * clicked, and its unread dot read a `read` field the daemon never sent, so
 * every line stayed unread for good.
 */
export function InboxNotificationItem({ notification }: InboxNotificationItemProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { markAsRead } = aos.stores.activity.useActions()
  const { body, createdAt, read } = notification
  const title = activityTitle(notification)

  const open = React.useCallback(() => {
    if (!read) {
      // Not awaited: the page opens either way, and a read receipt that did
      // not land is said rather than blocking the navigation on it.
      markAsRead(notification.id).catch((error: unknown) => {
        toast.error(t("Failed to mark activities as read"), { description: errorMessage(error) })
      })
    }
    void navigate(activityTarget(notification) as never)
  }, [markAsRead, navigate, notification, read, t])

  return (
    <button
      type="button"
      onClick={open}
      className={cn(
        "flex w-full gap-3 p-4 min-h-16 bg-background text-left hover:bg-muted/40 transition-colors",
        !read && "bg-muted/20",
      )}
    >
      <div className="flex-1 min-w-0 space-y-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className="text-xs leading-snug line-clamp-1" title={title}>{title}</p>
          <time dateTime={createdAt} className="text-xs text-muted-foreground shrink-0 tabular-nums">
            {activityTimeLabel(createdAt)}
          </time>
        </div>

        {body && (
          <p className="text-xs text-muted-foreground line-clamp-1">{body}</p>
        )}
      </div>

      {!read && (
        <span className="size-1.5 rounded-md bg-primary mt-1.5 shrink-0" aria-label={t("Unread")} role="img" />
      )}
    </button>
  )
}
