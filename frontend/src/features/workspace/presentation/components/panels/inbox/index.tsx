import * as React from "react"
import { motion } from "framer-motion"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { InboxHeader } from "./components/inbox-header.component"
import { InboxNotificationItem } from "./components/inbox-notification-item.component"
import { InboxEmpty } from "./components/inbox-empty.component"
import { aos } from "@/app/aos"
import { cn } from "@/lib/utils"
import { errorMessage } from "@/lib/aos-facade"
import { useTranslation } from "@/lib/i18n"
import type { ActivityEntry } from "@/features/activity/interfaces/activity.interfaces"
import { activityDayLabel } from "@/features/activity/presentation/helpers/activity-presentation.helper"

type InboxNotificationGroup = {
  label: string
  notifications: ActivityEntry[]
}

function groupNotificationsByDay(notifications: ActivityEntry[]) {
  const now = new Date()
  return notifications.reduce<InboxNotificationGroup[]>((groups, notification) => {
    const label = activityDayLabel(notification.createdAt, now)
    const group = groups[groups.length - 1]

    if (group?.label === label) {
      group.notifications.push(notification)
      return groups
    }

    groups.push({ label, notifications: [notification] })
    return groups
  }, [])
}

export function InboxPanel({ className }: { className?: string }) {
  const { t } = useTranslation()
  const activities = aos.stores.activity.useState((s) => s.activities)
  const total = aos.stores.activity.useState((s) => s.total)
  const { loadMore } = aos.stores.activity.useActions()
  const [isLoadingMore, setIsLoadingMore] = React.useState(false)
  const grouped = groupNotificationsByDay(activities)
  // "No more" only once there is no more: the list used to end with it after
  // the daemon's first page, with older entries out of reach.
  const hasMore = activities.length < total

  const handleLoadMore = React.useCallback(async () => {
    setIsLoadingMore(true)
    try {
      await loadMore()
    } catch (error) {
      toast.error(t("Failed to load activities"), { description: errorMessage(error) })
    } finally {
      setIsLoadingMore(false)
    }
  }, [loadMore, t])

  return (
    <motion.div
      className={cn("grid grid-rows-[auto_1fr] w-96 min-h-0", className)}
      animate={{ x: 0, opacity: 1 }}
      transition={{ duration: 0.2, ease: "easeInOut" }}
    >
      <InboxHeader />

      <div className="flex flex-col px-6 pb-6 h-full min-h-0 overflow-y-auto">
        {activities.length === 0 ? (
          <InboxEmpty />
        ) : (
          <div className="flex flex-col gap-4">
            {grouped.map((group) => (
              <div key={group.label} className="flex flex-col gap-2">
                <div className="px-1 text-[11px] text-muted-foreground">
                  {group.label}
                </div>

                <div className="flex flex-col border rounded-md divide-y overflow-hidden">
                  {group.notifications.map((notification) => (
                    <InboxNotificationItem key={notification.id} notification={notification} />
                  ))}
                </div>
              </div>
            ))}

            <div className="flex justify-center pb-1 pt-2">
              {hasMore ? (
                <Button
                  variant="outline"
                  size="sm"
                  className="rounded-full text-[11px]"
                  onClick={() => void handleLoadMore()}
                  disabled={isLoadingMore}
                  aria-busy={isLoadingMore}
                >
                  {isLoadingMore ? <Spinner /> : null}
                  {t("Load more")}
                </Button>
              ) : (
                <Badge variant="outline" className="rounded-full border-dashed text-[11px] font-medium text-muted-foreground">
                  {t("No more notifications")}
                </Badge>
              )}
            </div>
          </div>
        )}
      </div>
    </motion.div>
  )
}
