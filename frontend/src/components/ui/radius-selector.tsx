import { t } from "@/lib/i18n";
"use client"

import { cn } from "@/lib/utils"

export type ThemeRadius = "none" | "sm" | "md" | "lg"

export type ThemeRadiusOption = {
  value: ThemeRadius
  label: string
  radius: string
}

export const themeRadiusOptions: ThemeRadiusOption[] = [
  { value: "none", label: "None", radius: "0rem" },
  { value: "sm", label: "Small", radius: "0.375rem" },
  { value: "md", label: "Medium", radius: "0.75rem" },
  { value: "lg", label: "Large", radius: "1rem" },
]

const PREVIEW_BOX_SIZE_PX = 20
const RADIUS_REFERENCE_SIZE_PX = 32
const ROOT_FONT_SIZE_PX = 16

const radiusRemByValue: Record<ThemeRadius, number> = {
  none: 0,
  sm: 0.375,
  md: 0.75,
  lg: 1,
}

export function getRadiusPreviewBorderRadiusPx(
  radius: ThemeRadius,
  previewSizePx = PREVIEW_BOX_SIZE_PX,
  referenceSizePx = RADIUS_REFERENCE_SIZE_PX,
  rootFontSizePx = ROOT_FONT_SIZE_PX,
): number {
  const radiusPx = radiusRemByValue[radius] * rootFontSizePx
  if (radiusPx === 0) return 0

  return Math.round(radiusPx * (previewSizePx / referenceSizePx) * 10) / 10
}

const SETTINGS_CONTROL_WIDTH = "w-48"

type ThemeRadiusSelectorProps = {
  value: ThemeRadius
  onValueChange: (value: ThemeRadius) => void
  className?: string
  disabled?: boolean
}

export function ThemeRadiusSelector({
  value,
  onValueChange,
  className,
  disabled,
}: ThemeRadiusSelectorProps) {
  return (
    <div
      className={cn(
        "flex gap-1 rounded-md border border-border bg-muted/20 p-1 shadow-none",
        SETTINGS_CONTROL_WIDTH,
        className,
      )}
      role="radiogroup"
      aria-label={t("Corner radius")}
    >
      {themeRadiusOptions.map((option) => {
        const selected = value === option.value
        const previewRadiusPx = getRadiusPreviewBorderRadiusPx(option.value)

        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={disabled}
            onClick={() => onValueChange(option.value)}
            className={cn(
              "flex flex-1 flex-col items-center gap-1.5 rounded-md px-1.5 py-2 transition-colors outline-none focus-visible:ring-1 focus-visible:ring-ring/50 disabled:opacity-50",
              selected
                ? "bg-background text-foreground ring-1 ring-border"
                : "text-muted-foreground hover:bg-background/70 hover:text-foreground",
            )}
          >
            <span
              className={cn(
                "size-5 border border-border/80 bg-muted/50",
                selected && "border-primary/40 bg-primary/10",
              )}
              style={{ borderRadius: `${previewRadiusPx}px` }}
            />
            <span className="text-[10px] font-medium leading-none">{option.label}</span>
          </button>
        )
      })}
    </div>
  )
}
