import * as React from "react"

export interface SidebarActionButtonProps {
  icon: React.ComponentType<{ className?: string }>
  label: string
  disabled?: boolean
  onClick?: () => void
}

export function SidebarActionButton({
  icon: Icon,
  label,
  disabled,
  onClick,
}: SidebarActionButtonProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={(event) => {
        event.preventDefault()
        event.stopPropagation()
        onClick?.()
      }}
      aria-label={label}
      className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground disabled:pointer-events-none disabled:opacity-40"
      title={label}
    >
      <Icon className="size-3.5" />
    </button>
  )
}
