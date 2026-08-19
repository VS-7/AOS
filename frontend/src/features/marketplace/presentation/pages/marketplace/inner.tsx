import * as React from "react";
import { Link, useNavigate } from "@tanstack/react-router";

import type { FractalMarketplaceSkillListing } from "@/features/marketplace/interfaces/marketplace.interfaces";
import type { FractalSkill } from "@/features/skill/interfaces/skill.interfaces";
import {
  MarketplaceSidebar,
  type MarketplaceSidebarView,
} from "@/features/marketplace/presentation/components/marketplace-sidebar.component";
import { MarketplaceSearch } from "@/features/marketplace/presentation/components/marketplace-search.component";
import { MarketplaceShell } from "@/features/marketplace/presentation/components/marketplace-shell.component";
import { PluginSection } from "@/features/marketplace/presentation/components/plugin-section.component";
import {
  MARKETPLACE_ALLOWED_CATEGORIES,
  MARKETPLACE_FEATURED_SECTION_ID,
  MARKETPLACE_INSTALLED_SECTION_ID,
} from "@/features/marketplace/presentation/consts/marketplace";
import {
  filterListings,
  groupListingsByCategory,
  marketplaceSectionId,
  mergeListingsWithInstalled,
  pickFeaturedListings,
  resolveInstalledListing,
} from "@/features/marketplace/presentation/helpers/marketplace.helper";

interface MarketplacePageInnerProps {
  marketplacePlugins: FractalMarketplaceSkillListing[];
  installedPlugins: FractalSkill[];
  search: {
    category?: string;
    query?: string;
  };
}

export function MarketplacePageInner({
  marketplacePlugins,
  installedPlugins,
  search,
}: MarketplacePageInnerProps) {
  const navigate = useNavigate();
  const [activeView, setActiveView] =
    React.useState<MarketplaceSidebarView>("marketplace");
  const [searchQuery, setSearchQuery] = React.useState(search.query ?? "");

  React.useEffect(() => {
    void navigate({
      to: "/marketplace",
      search: (prev: { category?: string; query?: string }) => ({
        ...prev,
        query: searchQuery.trim() || undefined,
        category: undefined,
      }),
    });
  }, [searchQuery, navigate]);

  const isFiltered = Boolean(searchQuery.trim() || search.category);

  const filteredMarketplace = filterListings(marketplacePlugins, {
    query: searchQuery,
    category: search.category,
  });

  const listings = isFiltered
    ? mergeListingsWithInstalled(
        filteredMarketplace,
        installedPlugins,
        marketplacePlugins,
        searchQuery,
      )
    : filteredMarketplace;

  const featured = pickFeaturedListings(
    isFiltered ? listings : marketplacePlugins,
  );
  const featuredNames = React.useMemo(
    () => new Set(featured.map((listing) => listing.name)),
    [featured],
  );
  const grouped = groupListingsByCategory(listings);
  const categorySections = MARKETPLACE_ALLOWED_CATEGORIES.map(
    (categoryName) => ({
      id: marketplaceSectionId(categoryName),
      title: categoryName,
      listings: (grouped[categoryName] ?? []).filter(
        (listing) => !featuredNames.has(listing.name),
      ),
    }),
  ).filter((section) => section.listings.length > 0);

  const filteredInstalledListings = (
    searchQuery.trim()
      ? installedPlugins.filter((plugin) => {
          const query = searchQuery.toLowerCase();
          return (
            plugin.id.toLowerCase().includes(query) ||
            plugin.name.toLowerCase().includes(query) ||
            plugin.description.toLowerCase().includes(query)
          );
        })
      : installedPlugins
  ).map((plugin) =>
    resolveInstalledListing(
      plugin.id,
      plugin.description,
      marketplacePlugins,
    ),
  );

  const installedNames = React.useMemo(
    () =>
      new Set(
        installedPlugins.flatMap((plugin) => [plugin.id, plugin.name]),
      ),
    [installedPlugins],
  );

  const sidebarLinks = [
    ...(featured.length > 0
      ? [{ id: MARKETPLACE_FEATURED_SECTION_ID, label: "Featured Plugins" }]
      : []),
    ...categorySections.map((section) => ({
      id: section.id,
      label: section.title,
    })),
  ];

  function scrollToSection(id: string) {
    const target = document.getElementById(id);
    if (!target) return;
    target.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  return (
    <MarketplaceShell
      rail={
        <MarketplaceSidebar
          links={sidebarLinks}
          activeView={activeView}
          installedCount={installedPlugins.length}
          onSelectView={setActiveView}
          onScrollToSection={scrollToSection}
        />
      }
    >
      <div className="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
        <h1 className="text-2xl font-medium tracking-tight text-foreground md:text-[2rem] md:leading-none">
          Extend your Workspace
        </h1>
        <MarketplaceSearch
          defaultQuery={searchQuery}
          onQueryChange={setSearchQuery}
          className="w-full shrink-0 md:w-auto"
        />
      </div>

      {isFiltered ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[13px] text-muted-foreground">
            {listings.length} result{listings.length === 1 ? "" : "s"}
            {searchQuery ? ` for “${searchQuery}”` : ""}
            {search.category ? ` in ${search.category}` : ""}
          </span>
          <Link
            to="/marketplace"
            search={{}}
            className="text-[13px] font-medium text-foreground underline underline-offset-4"
          >
            Clear filters
          </Link>
        </div>
      ) : null}

      {activeView === "installed" ? (
        filteredInstalledListings.length > 0 ? (
          <PluginSection
            id={MARKETPLACE_INSTALLED_SECTION_ID}
            title="Installed Plugins"
            listings={filteredInstalledListings}
            installedNames={installedNames}
            previewLimit={filteredInstalledListings.length}
          />
        ) : (
          <p className="text-sm text-muted-foreground">
            {searchQuery.trim()
              ? "No installed plugins matched your search."
              : "No plugins installed yet. Browse the marketplace to install plugins."}
          </p>
        )
      ) : listings.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No plugins matched your search. Try another query or browse all
          categories.
        </p>
      ) : (
        <>
          {featured.length > 0 ? (
            <PluginSection
              id={MARKETPLACE_FEATURED_SECTION_ID}
              title="Featured Plugins"
              listings={featured}
              installedNames={installedNames}
              previewLimit={featured.length}
            />
          ) : null}

          {categorySections.map((section) => (
            <PluginSection
              key={section.id}
              id={section.id}
              title={section.title}
              listings={section.listings}
              installedNames={installedNames}
            />
          ))}
        </>
      )}
    </MarketplaceShell>
  );
}
