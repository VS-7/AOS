"use client";

import { MarketplaceRail } from "@/features/marketplace/presentation/components/marketplace-shell.component";
import { cn } from "@/lib/utils";

export type MarketplaceSidebarView = "marketplace" | "installed";

interface MarketplaceSidebarLink {
  id: string;
  label: string;
}

interface MarketplaceSidebarProps {
  links: MarketplaceSidebarLink[];
  activeView: MarketplaceSidebarView;
  installedCount: number;
  onSelectView: (view: MarketplaceSidebarView) => void;
  onScrollToSection: (id: string) => void;
}

export function MarketplaceSidebar({
  links,
  activeView,
  installedCount,
  onSelectView,
  onScrollToSection,
}: MarketplaceSidebarProps) {
  return (
    <MarketplaceRail>
      <nav aria-label="Marketplace sections" className="flex flex-col gap-4">
        <p className="text-[13px] font-medium text-foreground">Marketplace</p>

        <ul className="flex flex-col gap-1">
          {links.map((link) => (
            <li key={link.id}>
              <button
                type="button"
                onClick={() => {
                  onSelectView("marketplace");
                  onScrollToSection(link.id);
                }}
                className={cn(
                  "w-full rounded-md px-0 py-1.5 text-left text-[13px] transition-colors",
                  activeView === "marketplace"
                    ? "text-muted-foreground hover:text-foreground"
                    : "text-muted-foreground/70 hover:text-foreground",
                )}
              >
                {link.label}
              </button>
            </li>
          ))}
        </ul>

        <button
          type="button"
          onClick={() => onSelectView("installed")}
          className={cn(
            "w-full rounded-md px-0 py-1.5 text-left text-[13px] font-medium transition-colors",
            activeView === "installed"
              ? "text-foreground"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          Installed ({installedCount})
        </button>
      </nav>
    </MarketplaceRail>
  );
}
