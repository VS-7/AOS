import * as React from "react";
import { useNavigate } from "@tanstack/react-router";

import type {
  InstalledSkillRecord,
  MarketplaceSkillListing,
} from "@/features/marketplace/interfaces/marketplace.interfaces";
import {
  MarketplaceSidebar,
  type MarketplaceSidebarView,
} from "@/features/marketplace/presentation/components/marketplace-sidebar.component";
import { MarketplaceSearch } from "@/features/marketplace/presentation/components/marketplace-search.component";
import { MarketplaceShell } from "@/features/marketplace/presentation/components/marketplace-shell.component";
import { PluginSection } from "@/features/marketplace/presentation/components/plugin-section.component";
import { MarketplaceUnavailable } from "@/features/marketplace/presentation/components/marketplace-unavailable.component";
import { useTranslation } from "@/lib/i18n";
import {
  MARKETPLACE_ALLOWED_CATEGORIES,
  MARKETPLACE_FEATURED_SECTION_ID,
  MARKETPLACE_INSTALLED_SECTION_ID,
} from "@/features/marketplace/presentation/consts/marketplace";
import {
  MARKETPLACE_OTHER_CATEGORY,
  filterListings,
  groupListingsByCategory,
  listingForInstalled,
  marketplaceSectionId,
  pickFeaturedListings,
} from "@/features/marketplace/presentation/helpers/marketplace.helper";
import type { MarketplaceLoadError } from "./index";

interface MarketplacePageInnerProps {
  marketplacePlugins: MarketplaceSkillListing[];
  installedPlugins: InstalledSkillRecord[];
  marketplaceError: MarketplaceLoadError | null;
  installedError: MarketplaceLoadError | null;
  search: {
    category?: string;
    query?: string;
  };
}

function matchesInstalled(listing: MarketplaceSkillListing, query: string): boolean {
  if (!query) return true;
  return [listing.name, listing.displayName, listing.description].join(" ").toLowerCase().includes(query);
}

export function MarketplacePageInner({
  marketplacePlugins,
  installedPlugins,
  marketplaceError,
  installedError,
  search,
}: MarketplacePageInnerProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [activeView, setActiveView] =
    React.useState<MarketplaceSidebarView>("marketplace");
  const [searchQuery, setSearchQuery] = React.useState(search.query ?? "");
  const trimmedQuery = searchQuery.trim();

  // What the address says about the query, as far as this page knows: what it
  // read there, or last wrote there itself.
  const addressQuery = React.useRef(search.query ?? "");
  // Queries this page wrote that the address has not reported back yet. Each
  // lands a moment after it was written, while the person may have typed on:
  // taking one for a change made elsewhere put the box back to it.
  const unconfirmedWrites = React.useRef<string[]>([]);

  // The address is the source of truth for the query: a link to /marketplace
  // (the sidebar, Clear filters) resets what the box holds. It used to be
  // local state seeded once, so clearing the URL left the old search on
  // screen, "0 results for …" included. Its own writes coming back are not
  // such a change.
  React.useEffect(() => {
    const query = search.query ?? "";
    const own = unconfirmedWrites.current.indexOf(query);
    if (own !== -1) {
      unconfirmedWrites.current = unconfirmedWrites.current.slice(own + 1);
      return;
    }
    unconfirmedWrites.current = [];
    if (query === addressQuery.current) return;
    addressQuery.current = query;
    setSearchQuery(query);
  }, [search.query]);

  // …and the box writes to the address once typing pauses, only when it says
  // something new. This navigated on mount too, which dropped ?category=
  // before it could apply and re-ran the loader, so every visit searched every
  // registry twice.
  React.useEffect(() => {
    if (trimmedQuery === addressQuery.current) return;
    const timer = window.setTimeout(() => {
      addressQuery.current = trimmedQuery;
      unconfirmedWrites.current.push(trimmedQuery);
      void navigate({
        to: "/marketplace",
        search: (prev: { category?: string; query?: string }) => ({
          ...prev,
          query: trimmedQuery || undefined,
        }),
        replace: true,
      });
    }, 300);
    return () => window.clearTimeout(timer);
  }, [trimmedQuery, navigate]);

  // Through the address only: the effect above then empties the box, so the
  // two cannot disagree and the loader runs once.
  const clearFilters = () => {
    void navigate({
      to: "/marketplace",
      search: (prev: { category?: string; query?: string }) => ({
        ...prev,
        query: undefined,
        category: undefined,
      }),
    });
  };

  const isFiltered = Boolean(trimmedQuery || search.category);
  const normalizedQuery = trimmedQuery.toLowerCase();

  const installedListings = React.useMemo(
    () => installedPlugins.map((skill) => listingForInstalled(skill, marketplacePlugins)),
    [installedPlugins, marketplacePlugins],
  );

  const filteredMarketplace = filterListings(marketplacePlugins, {
    query: searchQuery,
    category: search.category,
  });

  // A search also finds what is installed and not in any registry.
  const listings = React.useMemo(() => {
    if (!isFiltered) return filteredMarketplace;
    const byName = new Map(filteredMarketplace.map((listing) => [listing.name, listing]));
    for (const listing of installedListings) {
      if (byName.has(listing.name)) continue;
      if (search.category && listing.category !== search.category) continue;
      if (!matchesInstalled(listing, normalizedQuery)) continue;
      byName.set(listing.name, listing);
    }
    return [...byName.values()];
  }, [filteredMarketplace, installedListings, isFiltered, normalizedQuery, search.category]);

  const featured = pickFeaturedListings(listings);
  const featuredNames = new Set(featured.map((listing) => listing.name));
  const grouped = groupListingsByCategory(listings);
  // "Other" is a section like the rest: it used to be counted in "1 result"
  // and never drawn, leaving the count above an empty page.
  const categorySections = [...MARKETPLACE_ALLOWED_CATEGORIES, MARKETPLACE_OTHER_CATEGORY]
    .map((categoryName) => ({
      id: marketplaceSectionId(categoryName),
      title: t(categoryName),
      listings: (grouped[categoryName] ?? []).filter(
        (listing) => !featuredNames.has(listing.name),
      ),
    }))
    .filter((section) => section.listings.length > 0);

  // The summary names the chosen category, so what is installed answers to it
  // as well: with no registry these are the only cards drawn, and they were
  // counted "in Development" whatever their own category was.
  const filteredInstalledListings = installedListings.filter(
    (listing) =>
      (!search.category || listing.category === search.category) &&
      matchesInstalled(listing, normalizedQuery),
  );

  const installedNames = React.useMemo(
    () =>
      new Set([
        ...installedPlugins.flatMap((plugin) => [plugin.id, plugin.name, plugin.source ?? ""]),
        ...installedListings.map((listing) => listing.name),
      ].filter(Boolean)),
    [installedPlugins, installedListings],
  );

  const sidebarLinks = [
    ...(featured.length > 0
      ? [{ id: MARKETPLACE_FEATURED_SECTION_ID, label: t("Featured Plugins") }]
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

  // What is actually drawn below: with no registry to search, only the
  // installed plugins are.
  const shownCount =
    activeView === "installed" || marketplaceError ? filteredInstalledListings.length : listings.length;
  let summary = shownCount === 1 ? t("1 result") : t("{{count}} results", { count: shownCount });
  if (trimmedQuery) summary = t("{{summary}} for “{{query}}”", { summary, query: trimmedQuery });
  if (search.category) summary = t("{{summary}} in {{category}}", { summary, category: t(search.category) });

  const installedSection =
    filteredInstalledListings.length > 0 ? (
      <PluginSection
        id={MARKETPLACE_INSTALLED_SECTION_ID}
        title={t("Installed Plugins")}
        listings={filteredInstalledListings}
        installedNames={installedNames}
        previewLimit={filteredInstalledListings.length}
      />
    ) : null;

  return (
    <MarketplaceShell
      rail={
        <MarketplaceSidebar
          links={marketplaceError ? [] : sidebarLinks}
          activeView={activeView}
          installedCount={installedPlugins.length}
          onSelectView={setActiveView}
          onScrollToSection={scrollToSection}
        />
      }
    >
      <div className="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
        <h1 className="text-2xl font-medium tracking-tight text-foreground md:text-[2rem] md:leading-none">
          {t("Extend your Workspace")}
        </h1>
        <MarketplaceSearch
          value={searchQuery}
          onValueChange={setSearchQuery}
          className="w-full shrink-0 md:w-auto"
        />
      </div>

      {isFiltered ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[13px] text-muted-foreground">{summary}</span>
          <button
            type="button"
            onClick={clearFilters}
            className="text-[13px] font-medium text-foreground underline underline-offset-4"
          >
            {t("Clear filters")}
          </button>
        </div>
      ) : null}

      {activeView === "installed" ? (
        installedError ? (
          <MarketplaceUnavailable code={installedError.code} message={installedError.message} />
        ) : installedSection ?? (
          <p className="text-sm text-muted-foreground">
            {isFiltered && installedListings.length > 0
              ? t("No installed plugins matched your search.")
              : t("No plugins installed yet. Browse the marketplace to install plugins.")}
          </p>
        )
      ) : marketplaceError ? (
        <>
          <MarketplaceUnavailable code={marketplaceError.code} message={marketplaceError.message} />
          {installedSection}
        </>
      ) : listings.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {isFiltered
            ? t("No plugins matched your search. Try another query or browse all categories.")
            : t("The configured registries list no plugins yet.")}
        </p>
      ) : (
        <>
          {featured.length > 0 ? (
            <PluginSection
              id={MARKETPLACE_FEATURED_SECTION_ID}
              title={t("Featured Plugins")}
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
