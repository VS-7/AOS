import * as React from "react"
import {
  ArrowRightLeft,
  UserCheck2,
  AtSign,
  CheckCheck,
  Bot,
  Zap,
  Download,
  Database,
  ListTodo,
  Trash2,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { NotificationPayload } from "@/core/builders/notification"
import { Link } from "@tanstack/react-router"

interface InboxNotificationItemProps {
  notification: NotificationPayload
}

function formatRelativeTime(dateStr: string) {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) return "Today"
  if (diffDays === 1) return "Yesterday"
  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" })
}

export function InboxNotificationItem({ notification }: InboxNotificationItemProps) {
  const { body, title, actions, read, createdAt, icon } = notification

  return (
    <div className={cn("flex gap-3 p-4 min-h-16 bg-background hover:bg-muted/40 transition-colors", !read && "bg-muted/20")}>
      <div className="flex-1 min-w-0 space-y-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className="text-xs leading-snug line-clamp-1">{title}</p>
          <span className="text-xs text-muted-foreground shrink-0 line-clamp-1">{formatRelativeTime(createdAt)}</span>
        </div>

        {body && (
          <p className="text-xs text-muted-foreground line-clamp-1">{body}</p>
        )}

        <div className="flex items-center gap-2">
          {actions?.map(action => {
            return (
              <Button variant="outline" size="sm" className="h-6 text-xs px-2 mt-2" asChild>
                <Link to={action.to}>
                  {action.label}
                </Link>
              </Button>
            )
          })}
        </div>
      </div>

      {!read && <span className="size-1.5 rounded-md bg-primary mt-1.5 shrink-0" />}
    </div>
  )
}
