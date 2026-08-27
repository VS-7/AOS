import { t } from "@/lib/i18n";
import * as React from "react"
import { motion } from "framer-motion"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { InboxHeader } from "./components/inbox-header.component"
import { InboxNotificationItem } from "./components/inbox-notification-item.component"
import { InboxEmpty } from "./components/inbox-empty.component"
import { aos } from "@/app/aos"
import { cn } from "@/lib/utils"
import { NotificationPayload } from "@/core/builders/notification"

type InboxNotificationGroup = {
  label: string
  notifications: NotificationPayload[]
}

function formatInboxDate(dateValue: string) {
  const date = new Date(dateValue)
  const today = new Date()

  const startOfToday = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  const startOfItemDay = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const diffDays = Math.round((startOfToday.getTime() - startOfItemDay.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays === 0) return "Today"
  if (diffDays === 1) return "Yesterday"

  return date.toLocaleDateString("en-US", {
    weekday: "long",
    month: "short",
    day: "numeric",
  })
}

function groupNotificationsByDay(notifications: NotificationPayload[]) {
  return notifications.reduce<InboxNotificationGroup[]>((groups, notification) => {
    const label = formatInboxDate(notification.createdAt)
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
  const activities = aos.stores.activity.useState((s) => s.activities)
  const grouped = groupNotificationsByDay(activities)

  return (
    <motion.div
      className={cn("grid grid-rows-[auto_1fr] w-96", className)}
      animate={{ x: 0, opacity: 1 }}
      transition={{ duration: 0.2, ease: "easeInOut" }}
    >
      <InboxHeader />

      <div className="flex flex-col px-6 h-full overflow-y-auto">
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
              <Badge variant="outline" className="rounded-full border-dashed text-[11px] font-medium text-muted-foreground">
                {t("No more notifications")}
              </Badge>
            </div>
          </div>
        )}
      </div>
    </motion.div>
  )
}
