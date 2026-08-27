"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface MarketplaceSearchProps {
  defaultQuery?: string;
  onQueryChange: (query: string) => void;
  className?: string;
}

export function MarketplaceSearch({
  defaultQuery = "",
  onQueryChange,
  className,
}: MarketplaceSearchProps) {
  const [value, setValue] = React.useState(defaultQuery);

  React.useEffect(() => {
    setValue(defaultQuery);
  }, [defaultQuery]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      onQueryChange(value.trim());
    }, 300);

    return () => window.clearTimeout(timer);
  }, [value, onQueryChange]);

  return (
    <input
      type="search"
      value={value}
      onChange={(event) => setValue(event.target.value)}
      placeholder={t("Search plugins...")}
      className={cn(
        "h-10 w-full rounded-md bg-[#f3f3f3] px-4 text-[13px] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-foreground/10 dark:bg-muted md:min-w-[280px] lg:min-w-[320px]",
        className,
      )}
    />
  );
}
