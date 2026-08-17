import { icons, LucideProps } from "lucide-react"

import { cn } from "@/lib/utils"

export interface IconProps extends LucideProps {
  /**
   * Lucide icon name (PascalCase), or an image URL / data URI.
   * @example "Clock", "Rocket", "data:image/webp;base64,..."
   */
  value?: string | null
  /**
   * Fallback Lucide icon name if the primary one is missing.
   * @default "Shapes"
   */
  fallback?: string
}

/**
 * Whether `value` should render as an image instead of a Lucide glyph.
 */
function isImageValue(value?: string | null): boolean {
  if (!value?.trim()) return false
  const trimmed = value.trim()
  return (
    trimmed.startsWith("data:image/") ||
    trimmed.startsWith("http://") ||
    trimmed.startsWith("https://") ||
    trimmed.startsWith("blob:")
  )
}

/**
 * Dynamic Icon component that resolves Lucide icons by string name,
 * or renders an `<img>` when `value` is an image URL / data URI.
 */
export function Icon({ value, fallback = "Shapes", className, ...props }: IconProps) {
  if (isImageValue(value)) {
    return (
      <img
        src={value!.trim()}
        alt=""
        className={cn("object-cover rounded-sm shrink-0", className)}
      />
    )
  }

  const IconComponent = (value && (icons as any)[value])
    ? (icons as any)[value]
    : (icons as any)[fallback]

  if (!IconComponent) {
    return null
  }

  return <IconComponent className={className} {...props} />
}
