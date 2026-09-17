"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface MarketplaceSearchProps {
  value: string;
  onValueChange: (value: string) => void;
  className?: string;
}

/**
 * The search box, controlled by the page.
 *
 * It used to keep its own copy, debounce it up to the page trimmed, and take
 * the page's answer back as its new value — a beat late, over whatever had
 * been typed meanwhile. Typing "demo crm plugin" a key every 300ms ended as
 * "dm cmpui", and a space at the end of a word never stayed. The page holds
 * the one value and decides when the address hears about it.
 */
export function MarketplaceSearch({ value, onValueChange, className }: MarketplaceSearchProps) {
  return (
    <input
      type="search"
      value={value}
      onChange={(event) => onValueChange(event.target.value)}
      placeholder={t("Search plugins...")}
      className={cn(
        "h-10 w-full rounded-md bg-[#f3f3f3] px-4 text-[13px] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-foreground/10 dark:bg-muted md:min-w-[280px] lg:min-w-[320px]",
        className,
      )}
    />
  );
}
