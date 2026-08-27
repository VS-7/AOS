import { t } from "@/lib/i18n";
import * as React from "react"
import { Inbox } from "lucide-react"

export function InboxEmpty() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-16 px-6 text-center">
      <Inbox className="size-8 text-muted-foreground/50" />
      <p className="text-sm font-medium text-muted-foreground">{t("You're all caught up")}</p>
      <p className="text-xs text-muted-foreground/70">{t("No new notifications")}</p>
    </div>
  )
}
