"use client";

import { forwardRef, type HTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";
import { useShape } from "@/lib/shape-context";

const badgeColors = {
  gray: "#a3a3a3",
  red: "#ef4444",
  orange: "#f97316",
  amber: "#f59e0b",
  yellow: "#eab308",
  lime: "#84cc16",
  green: "#22c55e",
  emerald: "#10b981",
  teal: "#14b8a6",
  cyan: "#06b6d4",
  blue: "#3b82f6",
  indigo: "#6366f1",
  violet: "#8b5cf6",
  purple: "#a855f7",
  fuchsia: "#d946ef",
  pink: "#ec4899",
  rose: "#f43f5e",
} as const;

type BadgeColor = keyof typeof badgeColors;

const badgeVariants = cva(
  "inline-flex items-center font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        default: "",
        secondary: "bg-secondary",
        destructive:
          "bg-destructive text-destructive-foreground border border-transparent",
        outline: "border border-border text-foreground",
      },
      size: {
        sm: "h-5 px-2 text-[11px] gap-1",
        md: "h-6 px-2.5 text-[12px] gap-1.5",
        lg: "h-7 px-3 text-[13px] gap-1.5",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "md",
    },
  },
);

interface BadgeProps
  extends
    Omit<HTMLAttributes<HTMLSpanElement>, "color">,
    VariantProps<typeof badgeVariants> {
  color?: BadgeColor;
}

const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  (
    {
      className,
      variant = "default",
      size = "md",
      color: _color = "gray",
      children,
      style,
      ...props
    },
    ref,
  ) => {
    const shape = useShape();

    return (
      <span
        ref={ref}
        className={cn(badgeVariants({ variant, size }), shape.item, className)}
        style={style}
        {...props}
      >
        {children}
      </span>
    );
  },
);

Badge.displayName = "Badge";

export { Badge, badgeVariants, badgeColors };
export type { BadgeProps, BadgeColor };
