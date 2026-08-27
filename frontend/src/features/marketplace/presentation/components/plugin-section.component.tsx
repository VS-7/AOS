"use client";

import * as React from "react";

import type { MarketplaceSkillListing } from "@/features/marketplace/interfaces/marketplace.interfaces";
import { MARKETPLACE_CATEGORY_PREVIEW_LIMIT } from "@/features/marketplace/presentation/consts/marketplace";
import { PluginCard } from "@/features/marketplace/presentation/components/plugin-card.component";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";

interface PluginSectionProps {
  id: string;
  title: string;
  listings: MarketplaceSkillListing[];
  installedNames?: ReadonlySet<string>;
  previewLimit?: number;
  className?: string;
}

export function PluginSection({
  id,
  title,
  listings,
  installedNames,
  previewLimit = MARKETPLACE_CATEGORY_PREVIEW_LIMIT,
  className,
}: PluginSectionProps) {
  const [expanded, setExpanded] = React.useState(false);

  if (listings.length === 0) return null;

  const hiddenCount = Math.max(0, listings.length - previewLimit);
  const visible = expanded ? listings : listings.slice(0, previewLimit);

  return (
    <section id={id} className={cn("scroll-mt-28", className)}>
      <h2 className="mb-5 text-xl font-medium tracking-tight text-foreground md:text-2xl">
        {title}
      </h2>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {visible.map((listing) => (
          <PluginCard
            key={listing.name}
            listing={listing}
            isInstalled={installedNames?.has(listing.name) ?? false}
          />
        ))}
      </div>

      {hiddenCount > 0 && !expanded ? (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="mt-4 text-[13px] font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          {t("View")} {hiddenCount} more
        </button>
      ) : null}
    </section>
  );
}
