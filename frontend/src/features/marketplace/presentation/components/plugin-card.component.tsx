import { Check, ChevronRight } from "lucide-react";
import { Link } from "@tanstack/react-router";

import type { MarketplaceSkillListing } from "@/features/marketplace/interfaces/marketplace.interfaces";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface PluginCardProps {
  listing: MarketplaceSkillListing;
  isInstalled?: boolean;
  className?: string;
}

export function PluginCard({
  listing,
  isInstalled = false,
  className,
}: PluginCardProps) {
  return (
    <Link
      to="/marketplace/$name"
      params={{ name: listing.name }}
      className={cn(
        "group flex items-center gap-3 rounded-md border border-border bg-card px-4 py-3.5 transition-colors hover:bg-muted/50",
        className,
      )}
    >
      <PluginLogo listing={listing} />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <h3 className="truncate text-[15px] font-medium leading-5 text-foreground">
            {listing.displayName}
          </h3>
          {isInstalled ? (
            <span
              className="inline-flex shrink-0 items-center gap-0.5 rounded-md bg-foreground/5 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
              title={t("Installed")}
            >
              <Check className="size-2.5" aria-hidden />
              {t("Installed")}
            </span>
          ) : null}
        </div>
        <p className="truncate text-[13px] leading-5 text-muted-foreground">
          {listing.shortDescription}
        </p>
      </div>
      <ChevronRight className="size-4 shrink-0 text-muted-foreground/70 transition-colors group-hover:text-muted-foreground" />
    </Link>
  );
}

export function PluginLogo({
  listing,
  size = 36,
}: {
  listing: Pick<
    MarketplaceSkillListing,
    "displayName" | "logo" | "brandColor" | "name"
  >;
  size?: number;
}) {
  if (listing.logo) {
    return (
      <div
        className="relative shrink-0 overflow-hidden rounded-md bg-white dark:bg-background"
        style={{ width: size, height: size }}
      >
        <img
          src={listing.logo}
          alt={`${listing.displayName} logo`}
          width={size}
          height={size}
          className="h-full w-full object-contain p-1.5 rounded-md"
          loading="lazy"
        />
      </div>
    );
  }

  return (
    <div
      className="flex shrink-0 items-center justify-center rounded-md bg-white text-[12px] font-semibold text-foreground dark:bg-background"
      style={{
        width: size,
        height: size,
        color: listing.brandColor ?? undefined,
      }}
    >
      {listing.displayName.slice(0, 2).toUpperCase()}
    </div>
  );
}
