import * as React from "react"
import { Collapsible as CollapsiblePrimitive } from "radix-ui"
import { ChevronRightIcon } from "lucide-react"

import { cn } from "@/lib/utils"

function Collapsible({
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.Root>) {
  return <CollapsiblePrimitive.Root data-slot="collapsible" {...props} />
}

function CollapsibleTrigger({
  className,
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.CollapsibleTrigger>) {
  return (
    <CollapsiblePrimitive.CollapsibleTrigger
      data-slot="collapsible-trigger"
      className={cn(className)}
      {...props}
    />
  )
}

function CollapsibleContent({
  className,
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.CollapsibleContent>) {
  return (
    <CollapsiblePrimitive.CollapsibleContent
      data-slot="collapsible-content"
      className={cn("w-full overflow-y-auto", className)}
      {...props}
    />
  )
}

function CollapsibleTitle({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="collapsible-title"
      className={cn("flex min-w-0 items-center gap-2 text-xs font-medium tracking-[0.08em] text-muted-foreground uppercase", className)}
      {...props}
    />
  )
}

function CollapsibleIcon({
  className,
  children,
  ...props
}: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="collapsible-icon"
      className={cn(
        "flex shrink-0 items-center justify-center text-muted-foreground transition-transform group-data-[state=open]/collapsible-trigger:rotate-90",
        className
      )}
      {...props}
    >
      {children ?? <ChevronRightIcon className="size-3.5" />}
    </span>
  )
}

function CollapsibleActions({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="collapsible-actions"
      className={cn("flex shrink-0 items-center gap-0.5", className)}
      {...props}
    />
  )
}

export {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
  CollapsibleTitle,
  CollapsibleIcon,
  CollapsibleActions,
}
