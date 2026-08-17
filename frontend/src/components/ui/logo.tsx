import * as React from "react";
import { cn } from "@/lib/utils";
import { useAppearance } from "@/lib/app-state";

/**
 * A placeholder mark, inline rather than a raster asset: there is no designed
 * AOS logo yet. Swap the <path> below when one exists — everything that reads
 * <Logo /> stays the same.
 */
export function Logo({
  className,
  ...props
}: React.SVGProps<SVGSVGElement>) {
  const appearance = useAppearance();
  void appearance; // the mark below is currentColor-based and needs no light/dark swap

  return (
    <svg
      viewBox="0 0 24 24"
      role="img"
      aria-label="AOS"
      className={cn("size-6", className)}
      {...props}
    >
      <rect x="2" y="2" width="20" height="20" rx="6" fill="currentColor" opacity="0.12" />
      <path
        d="M12 5 L18 18 H14.5 L12 12.5 L9.5 18 H6 Z"
        fill="currentColor"
      />
    </svg>
  );
}
