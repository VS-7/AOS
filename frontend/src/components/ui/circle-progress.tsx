import React from "react";
import { cn } from "@/lib/utils";

interface CircleProgressProps extends React.SVGProps<SVGSVGElement> {
  progress: number;
  size?: number;
  strokeWidth?: number;
  circleClassName?: string;
  progressClassName?: string;
}

export function CircleProgress({
  progress,
  size = 18,
  strokeWidth = 2,
  className,
  circleClassName,
  progressClassName,
  ...props
}: CircleProgressProps) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (Math.max(0, Math.min(100, progress)) / 100) * circumference;

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className={cn("shrink-0 -rotate-90", className)}
      {...props}
    >
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke="currentColor"
        strokeWidth={strokeWidth}
        className={cn("text-muted-foreground/20", circleClassName)}
      />
      {progress > 0 && (
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          className={cn("text-primary transition-all duration-300", progressClassName)}
        />
      )}
    </svg>
  );
}
