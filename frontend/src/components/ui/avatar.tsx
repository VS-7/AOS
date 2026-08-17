import * as React from "react"
import { Avatar as AvatarPrimitive } from "radix-ui"
import { Hashvatar } from 'hashvatar/react'
import { useQuery } from "@tanstack/react-query"

import { cn } from "@/lib/utils"
import { client } from "@/lib/client"

interface AgentSummary {
  id: string
  name: string
  image?: string | null
}

/**
 * AvatarAgentFallback displays an agent's avatar.
 *
 * Prefers a custom `image` (prop or resolved from `agents_list`). Falls back
 * to a deterministic Hashvatar when no image is set. The original also
 * animated on live occupancy (an agent mid-turn); AOS has no such signal on
 * the wire yet, so `animated` here is purely what the caller passes.
 */
function AvatarAgentFallback({
  name,
  size = 20,
  image,
  animated = false,
  className,
}: {
  name: string
  size?: number
  /** Optional avatar URL or data URI. When omitted, looks up the agent list. */
  image?: string | null
  animated?: boolean
  className?: string
}) {
  const agents = useQuery({
    queryKey: ["agents"],
    queryFn: async () =>
      (await client.invoke("agents_list", { _reasoning: "resolving an avatar" })) as {
        agents: AgentSummary[]
      },
  })
  const storeImage = agents.data?.agents.find(
    (agent) =>
      agent.id === name ||
      agent.name.toLowerCase() === name.toLowerCase(),
  )?.image
  // Explicit prop wins (including "" after remove). Otherwise use the list.
  const resolvedImage = (image !== undefined && image !== null
    ? image
    : storeImage
  )?.trim()

  if (resolvedImage) {
    return (
      <img
        src={resolvedImage}
        alt=""
        width={size}
        height={size}
        className={cn("size-full object-cover rounded-[inherit]", className)}
        style={{ width: size, height: size }}
      />
    )
  }

  return (
    <Hashvatar hash={name} size={size} mode="dither" animated={animated} className={className} />
  )
}

function Avatar({
  className,
  size = "default",
  ...props
}: React.ComponentProps<typeof AvatarPrimitive.Root> & {
  size?: "default" | "sm" | "lg"
}) {
  return (
    <AvatarPrimitive.Root
      data-slot="avatar"
      data-size={size}
      className={cn(
        "group/avatar relative flex size-8 shrink-0 overflow-hidden outline outline-input rounded-md select-none data-[size=lg]:size-10 data-[size=sm]:size-6",
        className
      )}
      {...props}
    />
  )
}

function AvatarImage({
  className,
  ...props
}: React.ComponentProps<typeof AvatarPrimitive.Image>) {
  return (
    <AvatarPrimitive.Image
      data-slot="avatar-image"
      className={cn("aspect-square size-full", className)}
      {...props}
    />
  )
}

function AvatarFallback({
  className,
  ...props
}: React.ComponentProps<typeof AvatarPrimitive.Fallback>) {
  return (
    <AvatarPrimitive.Fallback
      data-slot="avatar-fallback"
      className={cn(
        "flex size-full items-center justify-center rounded-md bg-muted text-sm text-muted-foreground group-data-[size=sm]/avatar:text-xs",
        className
      )}
      {...props}
    />
  )
}

function AvatarBadge({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="avatar-badge"
      className={cn(
        "absolute right-0 bottom-0 z-10 inline-flex items-center justify-center rounded-md bg-primary text-primary-foreground ring-2 ring-background select-none",
        "group-data-[size=sm]/avatar:size-2 group-data-[size=sm]/avatar:[&>svg]:hidden",
        "group-data-[size=default]/avatar:size-2.5 group-data-[size=default]/avatar:[&>svg]:size-2",
        "group-data-[size=lg]/avatar:size-3 group-data-[size=lg]/avatar:[&>svg]:size-2",
        className
      )}
      {...props}
    />
  )
}

function AvatarGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="avatar-group"
      className={cn(
        "group/avatar-group flex -space-x-2 *:data-[slot=avatar]:ring-2 *:data-[slot=avatar]:ring-background",
        className
      )}
      {...props}
    />
  )
}

function AvatarGroupCount({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="avatar-group-count"
      className={cn(
        "relative flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-sm text-muted-foreground ring-2 ring-background group-has-data-[size=lg]/avatar-group:size-10 group-has-data-[size=sm]/avatar-group:size-6 [&>svg]:size-4 group-has-data-[size=lg]/avatar-group:[&>svg]:size-5 group-has-data-[size=sm]/avatar-group:[&>svg]:size-3",
        className
      )}
      {...props}
    />
  )
}

export {
  Avatar,
  AvatarImage,
  AvatarFallback,
  AvatarBadge,
  AvatarGroup,
  AvatarGroupCount,
  AvatarAgentFallback,
}
